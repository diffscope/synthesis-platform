/**************************************************************************
 * DiffScope Synthesis Platform                                           *
 * Copyright (C) 2026 Team OpenVPI                                        *
 *                                                                        *
 * This program is free software: you can redistribute it and/or modify   *
 * it under the terms of the GNU General Public License as published by   *
 * the Free Software Foundation, either version 3 of the License, or      *
 * (at your option) any later version.                                    *
 *                                                                        *
 * This program is distributed in the hope that it will be useful,        *
 * but WITHOUT ANY WARRANTY; without even the implied warranty of         *
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the          *
 * GNU General Public License for more details.                           *
 *                                                                        *
 * You should have received a copy of the GNU General Public License      *
 * along with this program.  If not, see <https://www.gnu.org/licenses/>. *
 **************************************************************************/

package diffsinger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"diffscope-synthesis-platform/internal/api"
	"diffscope-synthesis-platform/internal/dsinfer"
	"diffscope-synthesis-platform/internal/dsinfer/builder"
	"diffscope-synthesis-platform/internal/utils"
)

const (
	durationSPPronunciation = "SP"
	durationAPPronunciation = "AP"
)

type durationPhonemeTarget struct {
	noteIndex    int
	phonemeIndex int
	noteStart    float64
	valid        bool
}

type durationPhonemeSource struct {
	target durationPhonemeTarget
	onset  bool
}

type durationWordMapping struct {
	start   float64
	targets []durationPhonemeTarget
}

type placedDurationNote struct {
	note  api.DurationNote
	start float64
	end   float64
}

var (
	durationInferenceTaskMu       sync.Mutex
	durationInferenceTaskResource = utils.NewResourceManager[*dsinfer.DurationInference, *dsinfer.DurationInferenceTask](
		0,
		0,
		func(_ *dsinfer.DurationInference, value *dsinfer.DurationInferenceTask) {
			value.Delete()
		},
	)
)

func (Architecture) Duration(
	ctx context.Context,
	archExtra json.RawMessage,
	singers []api.Singer,
	mix [][]float64,
	mixSampleRate float64,
	pieceDuration float64,
	notes []api.DurationNote,
) (<-chan api.DurationEvent, error) {
	extra, err := parseArchExtra(archExtra)
	if err != nil {
		return nil, err
	}
	_ = extra

	durationInference, speakerIDs, err := prepareDurationSingers(singers)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	builderNotes := convertDurationNotes(notes)
	words, err := builder.BuildWords(speakerIDs, mix, mixSampleRate, builderNotes)
	if err != nil {
		return nil, durationAPIError(err)
	}
	mapping, err := buildDurationWordMapping(notes)
	if err != nil {
		return nil, durationAPIError(err)
	}
	if countDurationPhones(words) != countDurationMappingPhones(mapping) {
		return nil, api.NewError(api.ProblemTypeInternalError, "duration word mapping shape does not match built words")
	}
	if len(words) == 0 {
		events := make(chan api.DurationEvent, 1)
		events <- api.DurationEvent{
			State:  api.StateComplete,
			Output: makeEmptyDurationOutput(notes),
		}
		close(events)
		return events, nil
	}

	events := make(chan api.DurationEvent, 4)
	go runDurationInference(ctx, events, durationInference, pieceDuration, words, mapping, notes)
	return events, nil
}

func configureDurationResourceManager() {
	durationInferenceTaskResource.SetTimeout(getInferenceCleanupTimeout())
	durationInferenceTaskResource.SetScanInterval(getInferenceCleanupInterval())
}

func prepareDurationSingers(singers []api.Singer) (*dsinfer.DurationInference, []string, error) {
	var durationInference *dsinfer.DurationInference
	var durationInferenceHandle uintptr
	var durationInferenceSinger string
	speakerIDs := make([]string, 0, len(singers))

	for _, singer := range singers {
		metadata, ok := getSingerMetadata(singer)
		if !ok {
			return nil, nil, api.NewSingerNotExistError(singer.ID, "")
		}
		if metadata.durationInference == nil || metadata.durationInference.Handle() == 0 {
			return nil, nil, api.NewSingerNotExistError(singer.ID, "")
		}

		extra, err := parseSingerExtra(singer.Extra)
		if err != nil {
			return nil, nil, err
		}
		if err := validateSingerExtraSpeaker(metadata, extra.Speaker); err != nil {
			return nil, nil, err
		}

		currentHandle := metadata.durationInference.Handle()
		if durationInference == nil {
			durationInference = metadata.durationInference
			durationInferenceHandle = currentHandle
			durationInferenceSinger = singer.ID
		} else if currentHandle != durationInferenceHandle {
			return nil, nil, api.NewSingersUnmixableError(
				durationInferenceSinger,
				singer.ID,
				"singers use different duration inference",
			)
		}

		speakerID, err := dsinfer.GetDurationInferenceSpeakerID(metadata.SynthRTSinger, extra.Speaker)
		if err != nil {
			return nil, nil, durationAPIError(err)
		}
		speakerIDs = append(speakerIDs, speakerID)
	}

	return durationInference, speakerIDs, nil
}

func convertDurationNotes(notes []api.DurationNote) []builder.Note {
	result := make([]builder.Note, 0, len(notes))
	for _, note := range notes {
		phonemes := make([]builder.Phoneme, 0, len(note.Phonemes))
		for _, phoneme := range note.Phonemes {
			phonemes = append(phonemes, builder.Phoneme{
				Token:    phoneme.Token,
				Onset:    phoneme.Onset,
				Language: phoneme.Language,
			})
		}
		result = append(result, builder.Note{
			Gap:           note.Position.Gap,
			Duration:      note.Position.Duration,
			Cents:         note.Cent,
			Pronunciation: note.Pronunciation,
			Language:      note.Language,
			Phonemes:      phonemes,
		})
	}
	return result
}

func runDurationInference(
	ctx context.Context,
	events chan<- api.DurationEvent,
	durationInference *dsinfer.DurationInference,
	pieceDuration float64,
	words []dsinfer.Word,
	mapping []durationWordMapping,
	notes []api.DurationNote,
) {
	defer close(events)

	lease, err := acquireResource(&durationInferenceTaskMu, durationInferenceTaskResource, durationInference, durationInference.CreateTask)
	if err != nil {
		sendDurationError(ctx, events, err)
		return
	}
	defer lease.Release()

	dsinferWords, err := dsinfer.NewWords(words)
	if err != nil {
		sendDurationError(ctx, events, err)
		return
	}

	run, err := lease.Value().Start(ctx, pieceDuration, dsinferWords)
	if err != nil {
		dsinferWords.Close()
		sendDurationError(ctx, events, err)
		return
	}

	if !sendDurationEvent(ctx, events, api.DurationEvent{State: api.StateQueuing}) {
		run.Terminate()
		<-run.Done()
		return
	}

	started := run.Started()
	done := run.Done()
	for started != nil || done != nil {
		if started != nil {
			select {
			case <-started:
				started = nil
				if !sendDurationEvent(ctx, events, api.DurationEvent{State: api.StateProcessing}) {
					run.Terminate()
					<-done
					return
				}
				continue
			default:
			}
		}

		select {
		case <-ctx.Done():
			run.Terminate()
			<-done
			return
		case <-started:
			started = nil
			if !sendDurationEvent(ctx, events, api.DurationEvent{State: api.StateProcessing}) {
				run.Terminate()
				<-done
				return
			}
		case result := <-done:
			done = nil
			if result.Err != nil {
				sendDurationError(ctx, events, result.Err)
				return
			}

			output, err := makeDurationOutput(notes, mapping, result.Durations)
			if err != nil {
				sendDurationError(ctx, events, err)
				return
			}
			sendDurationEvent(ctx, events, api.DurationEvent{
				State:  api.StateComplete,
				Output: output,
			})
			return
		}
	}
}

func sendDurationEvent(ctx context.Context, events chan<- api.DurationEvent, event api.DurationEvent) bool {
	if err := ctx.Err(); err != nil {
		return false
	}
	select {
	case events <- event:
		return true
	case <-ctx.Done():
		return false
	}
}

func sendDurationError(ctx context.Context, events chan<- api.DurationEvent, err error) {
	if err == nil || ctx.Err() != nil {
		return
	}
	sendDurationEvent(ctx, events, api.DurationEvent{
		State: api.StateError,
		Err:   durationAPIError(err),
	})
}

func makeEmptyDurationOutput(notes []api.DurationNote) api.DurationOutput {
	result := api.DurationOutput{
		Notes: make([]api.DurationOutputNote, len(notes)),
	}
	for noteIndex, note := range notes {
		result.Notes[noteIndex] = api.DurationOutputNote{
			Phonemes: make([]api.DurationOutputPhoneme, len(note.Phonemes)),
		}
	}
	return result
}

func makeDurationOutput(
	notes []api.DurationNote,
	mapping []durationWordMapping,
	durations []float64,
) (api.DurationOutput, error) {
	if len(durations) != countDurationMappingPhones(mapping) {
		return api.DurationOutput{}, api.NewError(
			api.ProblemTypeInternalError,
			fmt.Sprintf("duration result count mismatch: expected %d, got %d", countDurationMappingPhones(mapping), len(durations)),
		)
	}

	output := makeEmptyDurationOutput(notes)
	durationIndex := 0
	for _, word := range mapping {
		cursor := 0.0
		for _, target := range word.targets {
			if target.valid {
				output.Notes[target.noteIndex].Phonemes[target.phonemeIndex].Start = word.start + cursor - target.noteStart
			}
			cursor += durations[durationIndex]
			durationIndex++
		}
	}
	return output, nil
}

func buildDurationWordMapping(notes []api.DurationNote) ([]durationWordMapping, error) {
	placedNotes, err := placeDurationNotes(notes)
	if err != nil {
		return nil, err
	}
	if len(placedNotes) == 0 {
		return []durationWordMapping{}, nil
	}
	if isDurationSlur(placedNotes[0].note) {
		return nil, fmt.Errorf("first note cannot be slur")
	}

	result := make([]durationWordMapping, 0, len(placedNotes))
	if placedNotes[0].note.Position.Gap > 0 {
		header, _ := splitDurationHeaderAndBody(durationPhonemesOf(0, placedNotes[0]))
		targets := make([]durationPhonemeTarget, 0, len(header)+1)
		targets = append(targets, durationPhonemeTarget{})
		targets = appendDurationTargets(targets, header)
		result = appendDurationWordMapping(result, 0, targets)
	}

	for noteIndex := 0; noteIndex < len(placedNotes); noteIndex++ {
		note := placedNotes[noteIndex]
		wordStart := note.start
		wordEnd := note.end
		_, body := splitDurationHeaderAndBody(durationPhonemesOf(noteIndex, note))
		wordTargets := durationTargetsOf(body)

		for noteIndex+1 < len(placedNotes) && isDurationSlur(placedNotes[noteIndex+1].note) {
			wordEnd = placedNotes[noteIndex+1].end
			noteIndex++
		}

		if noteIndex+1 >= len(placedNotes) {
			result = appendDurationWordMapping(result, wordStart, wordTargets)
			continue
		}

		next := placedNotes[noteIndex+1]
		gapLength := next.start - wordEnd
		if gapLength < 0 {
			gapLength = 0
		}
		header, _ := splitDurationHeaderAndBody(durationPhonemesOf(noteIndex+1, next))

		if gapLength == 0 {
			wordTargets = appendDurationTargets(wordTargets, header)
		}
		result = appendDurationWordMapping(result, wordStart, wordTargets)
		if gapLength > 0 {
			gapTargets := make([]durationPhonemeTarget, 0, len(header)+1)
			gapTargets = append(gapTargets, durationPhonemeTarget{})
			gapTargets = appendDurationTargets(gapTargets, header)
			result = appendDurationWordMapping(result, wordEnd, gapTargets)
		}
	}

	return result, nil
}

func placeDurationNotes(notes []api.DurationNote) ([]placedDurationNote, error) {
	result := make([]placedDurationNote, 0, len(notes))
	var position float64
	for index, note := range notes {
		if note.Position.Gap < 0 {
			return nil, fmt.Errorf("note %d gap cannot be negative", index)
		}
		if note.Position.Duration < 0 {
			return nil, fmt.Errorf("note %d duration cannot be negative", index)
		}
		position += note.Position.Gap
		placed := placedDurationNote{
			note:  note,
			start: position,
			end:   position + note.Position.Duration,
		}
		result = append(result, placed)
		position = placed.end
	}
	return result, nil
}

func durationPhonemesOf(noteIndex int, note placedDurationNote) []durationPhonemeSource {
	if len(note.note.Phonemes) == 0 && isDurationRest(note.note) {
		return []durationPhonemeSource{{onset: true}}
	}

	result := make([]durationPhonemeSource, 0, len(note.note.Phonemes))
	for phonemeIndex, phoneme := range note.note.Phonemes {
		result = append(result, durationPhonemeSource{
			target: durationPhonemeTarget{
				noteIndex:    noteIndex,
				phonemeIndex: phonemeIndex,
				noteStart:    note.start,
				valid:        true,
			},
			onset: phoneme.Onset,
		})
	}
	return result
}

func splitDurationHeaderAndBody(phonemes []durationPhonemeSource) ([]durationPhonemeSource, []durationPhonemeSource) {
	for index, phoneme := range phonemes {
		if phoneme.onset {
			return phonemes[:index], phonemes[index:]
		}
	}
	return phonemes, nil
}

func durationTargetsOf(phonemes []durationPhonemeSource) []durationPhonemeTarget {
	result := make([]durationPhonemeTarget, 0, len(phonemes))
	return appendDurationTargets(result, phonemes)
}

func appendDurationTargets(targets []durationPhonemeTarget, phonemes []durationPhonemeSource) []durationPhonemeTarget {
	for _, phoneme := range phonemes {
		targets = append(targets, phoneme.target)
	}
	return targets
}

func appendDurationWordMapping(
	mapping []durationWordMapping,
	start float64,
	targets []durationPhonemeTarget,
) []durationWordMapping {
	if len(targets) == 0 {
		return mapping
	}
	return append(mapping, durationWordMapping{
		start:   start,
		targets: targets,
	})
}

func countDurationPhones(words []dsinfer.Word) int {
	var count int
	for _, word := range words {
		count += len(word.Phonemes)
	}
	return count
}

func countDurationMappingPhones(mapping []durationWordMapping) int {
	var count int
	for _, word := range mapping {
		count += len(word.targets)
	}
	return count
}

func isDurationRest(note api.DurationNote) bool {
	return note.Pronunciation == durationSPPronunciation || note.Pronunciation == durationAPPronunciation
}

func isDurationSlur(note api.DurationNote) bool {
	return note.Pronunciation == slurPronunciation
}

func durationAPIError(err error) error {
	if err == nil {
		return nil
	}
	var apiError *api.Error
	if errors.As(err, &apiError) {
		return err
	}
	return api.NewError(api.ProblemTypeInternalError, err.Error())
}
