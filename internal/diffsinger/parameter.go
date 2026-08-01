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
	parameterIDPitch          = "pitch"
	parameterIDExpressiveness = "expressiveness"
	parameterIDEnergy         = "energy"
	parameterIDBreathiness    = "breathiness"
	parameterIDVoicing        = "voicing"
	parameterIDTension        = "tension"
	parameterIDMouthOpening   = "mouth_opening"
)

var (
	varianceParameterIDs = []string{
		parameterIDEnergy,
		parameterIDBreathiness,
		parameterIDVoicing,
		parameterIDTension,
		parameterIDMouthOpening,
	}

	varianceParameterTagsByID = map[string]dsinfer.ParameterTag{
		parameterIDEnergy:       dsinfer.ParameterTagEnergy,
		parameterIDBreathiness:  dsinfer.ParameterTagBreathiness,
		parameterIDVoicing:      dsinfer.ParameterTagVoicing,
		parameterIDTension:      dsinfer.ParameterTagTension,
		parameterIDMouthOpening: dsinfer.ParameterTagMouthOpening,
	}

	parameterPitchInferenceTaskMu       sync.Mutex
	parameterPitchInferenceTaskResource = utils.NewResourceManager[*dsinfer.PitchInference, *dsinfer.PitchInferenceTask](
		0,
		0,
		func(_ *dsinfer.PitchInference, value *dsinfer.PitchInferenceTask) {
			value.Delete()
		},
	)

	parameterVarianceInferenceTaskMu       sync.Mutex
	parameterVarianceInferenceTaskResource = utils.NewResourceManager[*dsinfer.VarianceInference, *dsinfer.VarianceInferenceTask](
		0,
		0,
		func(_ *dsinfer.VarianceInference, value *dsinfer.VarianceInferenceTask) {
			value.Delete()
		},
	)
)

type parameterPlan struct {
	pitchRetake       bool
	varianceRetakes   map[string]bool
	pitchInference    *dsinfer.PitchInference
	pitchSpeakers     []string
	varianceInference *dsinfer.VarianceInference
	varianceSpeakers  []string
}

func (p parameterPlan) needsPitch() bool {
	return p.pitchRetake
}

func (p parameterPlan) needsVariance() bool {
	return len(p.varianceRetakes) > 0
}

func (Architecture) Parameter(
	ctx context.Context,
	archExtra json.RawMessage,
	singers []api.Singer,
	mix [][]float64,
	mixSampleRate float64,
	pieceDuration float64,
	notes []api.Note,
	parameters map[string]api.Parameter,
) (<-chan api.ParameterEvent, error) {
	extra, err := parseArchExtra(archExtra)
	if err != nil {
		return nil, err
	}

	plan, err := buildParameterPlan(singers, parameters)
	if err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !plan.needsPitch() && !plan.needsVariance() {
		return makeEmptyParameterEvents(), nil
	}

	var pitchWords []dsinfer.Word
	var pitchDynamicMixedSpeakers []dsinfer.DynamicMixedSpeaker
	var pitchParameters []dsinfer.Parameter
	if plan.needsPitch() {
		pitchWords, err = builder.BuildWords(plan.pitchSpeakers, mix, mixSampleRate, convertParameterNotes(notes))
		if err != nil {
			return nil, parameterAPIError(err)
		}
		pitchDynamicMixedSpeakers, err = builder.BuildDynamicMixedSpeakers(plan.pitchSpeakers, mixSampleRate, mix)
		if err != nil {
			return nil, parameterAPIError(err)
		}
		pitchParameters, err = buildPitchParameters(parameters)
		if err != nil {
			return nil, err
		}
	}

	var varianceWords []dsinfer.Word
	var varianceDynamicMixedSpeakers []dsinfer.DynamicMixedSpeaker
	if plan.needsVariance() {
		varianceWords, err = builder.BuildWords(plan.varianceSpeakers, mix, mixSampleRate, convertParameterNotes(notes))
		if err != nil {
			return nil, parameterAPIError(err)
		}
		varianceDynamicMixedSpeakers, err = builder.BuildDynamicMixedSpeakers(plan.varianceSpeakers, mixSampleRate, mix)
		if err != nil {
			return nil, parameterAPIError(err)
		}
		if !plan.needsPitch() {
			if _, ok := parameters[parameterIDPitch]; !ok {
				return nil, newInvalidParameterError(parameterIDPitch, "missing", "missing pitch parameter")
			}
		}
	}

	events := make(chan api.ParameterEvent, 4)
	go runParameterInference(
		ctx,
		events,
		extra.Steps,
		pieceDuration,
		parameters,
		plan,
		pitchWords,
		pitchParameters,
		pitchDynamicMixedSpeakers,
		varianceWords,
		varianceDynamicMixedSpeakers,
	)
	return events, nil
}

func configureParameterResourceManager() {
	parameterPitchInferenceTaskResource.SetTimeout(getInferenceCleanupTimeout())
	parameterPitchInferenceTaskResource.SetScanInterval(getInferenceCleanupInterval())
	parameterVarianceInferenceTaskResource.SetTimeout(getInferenceCleanupTimeout())
	parameterVarianceInferenceTaskResource.SetScanInterval(getInferenceCleanupInterval())
}

func buildParameterPlan(singers []api.Singer, parameters map[string]api.Parameter) (parameterPlan, error) {
	plan := parameterPlan{
		varianceRetakes: make(map[string]bool),
	}
	for id, parameter := range parameters {
		if parameter.Retake == nil {
			continue
		}
		switch id {
		case parameterIDPitch:
			plan.pitchRetake = true
		case parameterIDEnergy, parameterIDBreathiness, parameterIDVoicing, parameterIDTension, parameterIDMouthOpening:
			plan.varianceRetakes[id] = true
		default:
			return parameterPlan{}, newInvalidParameterError(
				id,
				"retake_not_supported",
				fmt.Sprintf("parameter %q does not support retake", id),
			)
		}
	}
	if plan.needsPitch() {
		pitchInference, speakerIDs, err := prepareParameterPitchSingers(singers)
		if err != nil {
			return parameterPlan{}, err
		}
		plan.pitchInference = pitchInference
		plan.pitchSpeakers = speakerIDs
	}
	if plan.needsVariance() {
		varianceInference, speakerIDs, err := prepareParameterVarianceSingers(singers)
		if err != nil {
			return parameterPlan{}, err
		}
		plan.varianceInference = varianceInference
		plan.varianceSpeakers = speakerIDs
	}
	return plan, nil
}

func prepareParameterPitchSingers(singers []api.Singer) (*dsinfer.PitchInference, []string, error) {
	var pitchInference *dsinfer.PitchInference
	var pitchInferenceHandle uintptr
	var pitchInferenceSinger string
	speakerIDs := make([]string, 0, len(singers))

	for _, singer := range singers {
		metadata, ok := getSingerMetadata(singer)
		if !ok {
			return nil, nil, api.NewSingerNotExistError(singer.ID, "")
		}
		if metadata.pitchInference == nil || metadata.pitchInference.Handle() == 0 {
			return nil, nil, api.NewSingerNotExistError(singer.ID, "")
		}

		extra, err := parseSingerExtra(singer.Extra)
		if err != nil {
			return nil, nil, err
		}
		if err := validateSingerExtraSpeaker(metadata, extra.Speaker); err != nil {
			return nil, nil, err
		}

		currentHandle := metadata.pitchInference.Handle()
		if pitchInference == nil {
			pitchInference = metadata.pitchInference
			pitchInferenceHandle = currentHandle
			pitchInferenceSinger = singer.ID
		} else if currentHandle != pitchInferenceHandle {
			return nil, nil, api.NewSingersUnmixableError(
				pitchInferenceSinger,
				singer.ID,
				"singers use different pitch inference",
			)
		}

		speakerID, err := dsinfer.GetPitchInferenceSpeakerID(metadata.SynthRTSinger, extra.Speaker)
		if err != nil {
			return nil, nil, parameterAPIError(err)
		}
		speakerIDs = append(speakerIDs, speakerID)
	}

	return pitchInference, speakerIDs, nil
}

func prepareParameterVarianceSingers(singers []api.Singer) (*dsinfer.VarianceInference, []string, error) {
	var varianceInference *dsinfer.VarianceInference
	var varianceInferenceHandle uintptr
	var varianceInferenceSinger string
	speakerIDs := make([]string, 0, len(singers))

	for _, singer := range singers {
		metadata, ok := getSingerMetadata(singer)
		if !ok {
			return nil, nil, api.NewSingerNotExistError(singer.ID, "")
		}
		if metadata.varianceInference == nil || metadata.varianceInference.Handle() == 0 {
			return nil, nil, api.NewSingerNotExistError(singer.ID, "")
		}

		extra, err := parseSingerExtra(singer.Extra)
		if err != nil {
			return nil, nil, err
		}
		if err := validateSingerExtraSpeaker(metadata, extra.Speaker); err != nil {
			return nil, nil, err
		}

		currentHandle := metadata.varianceInference.Handle()
		if varianceInference == nil {
			varianceInference = metadata.varianceInference
			varianceInferenceHandle = currentHandle
			varianceInferenceSinger = singer.ID
		} else if currentHandle != varianceInferenceHandle {
			return nil, nil, api.NewSingersUnmixableError(
				varianceInferenceSinger,
				singer.ID,
				"singers use different variance inference",
			)
		}

		speakerID, err := dsinfer.GetVarianceInferenceSpeakerID(metadata.SynthRTSinger, extra.Speaker)
		if err != nil {
			return nil, nil, parameterAPIError(err)
		}
		speakerIDs = append(speakerIDs, speakerID)
	}

	return varianceInference, speakerIDs, nil
}

func convertParameterNotes(notes []api.Note) []builder.Note {
	result := make([]builder.Note, 0, len(notes))
	for _, note := range notes {
		phonemes := make([]builder.Phoneme, 0, len(note.Phonemes))
		for _, phoneme := range note.Phonemes {
			phonemes = append(phonemes, builder.Phoneme{
				Token:    phoneme.Token,
				Start:    phoneme.Start,
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

func buildPitchParameters(parameters map[string]api.Parameter) ([]dsinfer.Parameter, error) {
	expressiveness, ok := parameters[parameterIDExpressiveness]
	if !ok {
		return nil, newInvalidParameterError(parameterIDExpressiveness, "missing", "missing expressiveness parameter")
	}
	pitch, ok := parameters[parameterIDPitch]
	if !ok {
		return nil, newInvalidParameterError(parameterIDPitch, "missing", "missing pitch parameter")
	}
	if pitch.Retake == nil {
		return nil, newInvalidParameterError(parameterIDPitch, "retake_required", "missing pitch retake")
	}

	expressivenessParameter, err := buildParameter(parameterIDExpressiveness, expressiveness, false)
	if err != nil {
		return nil, err
	}
	pitchParameter, err := buildParameter(parameterIDPitch, pitch, true)
	if err != nil {
		return nil, err
	}
	return []dsinfer.Parameter{expressivenessParameter, pitchParameter}, nil
}

func buildVarianceParameters(
	parameters map[string]api.Parameter,
	varianceRetakes map[string]bool,
	pitch *dsinfer.Parameter,
) ([]dsinfer.Parameter, error) {
	result := make([]dsinfer.Parameter, 0, len(varianceRetakes)+1)
	if pitch != nil {
		result = append(result, dsinfer.Parameter{
			Tag:      pitch.Tag,
			Values:   append([]float64(nil), pitch.Values...),
			Interval: pitch.Interval,
		})
	} else {
		pitchParameter, ok := parameters[parameterIDPitch]
		if !ok {
			return nil, newInvalidParameterError(parameterIDPitch, "missing", "missing pitch parameter")
		}
		parameter, err := buildParameter(parameterIDPitch, pitchParameter, false)
		if err != nil {
			return nil, err
		}
		result = append(result, parameter)
	}

	for _, id := range varianceParameterIDs {
		if !varianceRetakes[id] {
			continue
		}
		parameter, err := buildParameter(id, parameters[id], true)
		if err != nil {
			return nil, err
		}
		result = append(result, parameter)
	}
	return result, nil
}

func buildParameter(id string, parameter api.Parameter, retake bool) (dsinfer.Parameter, error) {
	retakePosition := 0
	retakeLength := 0
	if retake {
		if parameter.Retake == nil {
			return dsinfer.Parameter{}, newInvalidParameterError(id, "retake_required", fmt.Sprintf("missing %s retake", id))
		}
		retakePosition = parameter.Retake.Position
		retakeLength = parameter.Retake.Length
	}
	values := parameterValuesOrDefault(id, parameter.Values)
	result, err := builder.BuildParameter(id, parameter.SampleRate, values, retake, retakePosition, retakeLength)
	if err != nil {
		return dsinfer.Parameter{}, newInvalidParameterError(id, "invalid_value", err.Error())
	}
	return result, nil
}

func parameterValuesOrDefault(id string, values []float64) []float64 {
	if len(values) > 0 {
		return values
	}
	if id == parameterIDExpressiveness {
		return []float64{1000}
	}
	return []float64{0}
}

func runParameterInference(
	ctx context.Context,
	events chan<- api.ParameterEvent,
	steps int64,
	pieceDuration float64,
	inputParameters map[string]api.Parameter,
	plan parameterPlan,
	pitchWords []dsinfer.Word,
	pitchParameters []dsinfer.Parameter,
	pitchDynamicMixedSpeakers []dsinfer.DynamicMixedSpeaker,
	varianceWords []dsinfer.Word,
	varianceDynamicMixedSpeakers []dsinfer.DynamicMixedSpeaker,
) {
	defer close(events)

	output := api.ParameterOutput{
		Parameters: map[string]api.ParameterOutputParameter{},
	}
	queued := false
	var pitch *dsinfer.Parameter

	if plan.needsPitch() {
		result, ok := runPitchParameterInference(
			ctx,
			events,
			plan.pitchInference,
			pieceDuration,
			pitchWords,
			pitchParameters,
			pitchDynamicMixedSpeakers,
			steps,
			&queued,
		)
		if !ok {
			return
		}
		if result.Err != nil {
			sendParameterError(ctx, events, result.Err)
			return
		}
		pitch = &result.Pitch

		id, values, sampleRate, err := builder.ParseParameter(result.Pitch)
		if err != nil {
			sendParameterError(ctx, events, err)
			return
		}
		if id != parameterIDPitch {
			sendParameterError(ctx, events, api.NewError(api.ProblemTypeInternalError, "pitch result tag mismatch"))
			return
		}
		output.Parameters[parameterIDPitch] = api.ParameterOutputParameter{
			Values:     values,
			SampleRate: sampleRate,
		}
		if plan.needsVariance() && !sendParameterEvent(ctx, events, api.ParameterEvent{
			State:  api.StateProcessing,
			Output: api.ParameterOutput{Parameters: cloneParameterOutput(output.Parameters)},
		}) {
			return
		}
	}

	if plan.needsVariance() {
		varianceParameters, err := buildVarianceParameters(
			inputParameters,
			plan.varianceRetakes,
			pitch,
		)
		if err != nil {
			sendParameterError(ctx, events, err)
			return
		}
		result, ok := runVarianceParameterInference(
			ctx,
			events,
			plan.varianceInference,
			pieceDuration,
			varianceWords,
			varianceParameters,
			varianceDynamicMixedSpeakers,
			steps,
			&queued,
		)
		if !ok {
			return
		}
		if result.Err != nil {
			sendParameterError(ctx, events, result.Err)
			return
		}

		for id, parameter := range parseVarianceParameters(result.Parameters, plan.varianceRetakes, inputParameters) {
			output.Parameters[id] = parameter
		}
	}

	sendParameterEvent(ctx, events, api.ParameterEvent{
		State:  api.StateComplete,
		Output: output,
	})
}

func runPitchParameterInference(
	ctx context.Context,
	events chan<- api.ParameterEvent,
	pitchInference *dsinfer.PitchInference,
	pieceDuration float64,
	words []dsinfer.Word,
	parameters []dsinfer.Parameter,
	dynamicMixedSpeakers []dsinfer.DynamicMixedSpeaker,
	steps int64,
	queued *bool,
) (dsinfer.PitchInferenceResult, bool) {
	lease, err := acquireResource(&parameterPitchInferenceTaskMu, parameterPitchInferenceTaskResource, pitchInference, pitchInference.CreateTask)
	if err != nil {
		sendParameterError(ctx, events, err)
		return dsinfer.PitchInferenceResult{}, false
	}
	defer lease.Release()

	dsinferWords, dsinferParameters, dsinferDynamicMixedSpeakers, err := newParameterInferenceInput(
		words,
		parameters,
		dynamicMixedSpeakers,
	)
	if err != nil {
		sendParameterError(ctx, events, err)
		return dsinfer.PitchInferenceResult{}, false
	}

	run, err := lease.Value().Start(ctx, pieceDuration, dsinferWords, dsinferParameters, dsinferDynamicMixedSpeakers, steps)
	if err != nil {
		dsinferWords.Close()
		dsinferParameters.Close()
		dsinferDynamicMixedSpeakers.Close()
		sendParameterError(ctx, events, err)
		return dsinfer.PitchInferenceResult{}, false
	}

	return waitPitchParameterInference(ctx, events, run, queued)
}

func runVarianceParameterInference(
	ctx context.Context,
	events chan<- api.ParameterEvent,
	varianceInference *dsinfer.VarianceInference,
	pieceDuration float64,
	words []dsinfer.Word,
	parameters []dsinfer.Parameter,
	dynamicMixedSpeakers []dsinfer.DynamicMixedSpeaker,
	steps int64,
	queued *bool,
) (dsinfer.VarianceInferenceResult, bool) {
	lease, err := acquireResource(&parameterVarianceInferenceTaskMu, parameterVarianceInferenceTaskResource, varianceInference, varianceInference.CreateTask)
	if err != nil {
		sendParameterError(ctx, events, err)
		return dsinfer.VarianceInferenceResult{}, false
	}
	defer lease.Release()

	dsinferWords, dsinferParameters, dsinferDynamicMixedSpeakers, err := newParameterInferenceInput(
		words,
		parameters,
		dynamicMixedSpeakers,
	)
	if err != nil {
		sendParameterError(ctx, events, err)
		return dsinfer.VarianceInferenceResult{}, false
	}

	run, err := lease.Value().Start(ctx, pieceDuration, dsinferWords, dsinferParameters, dsinferDynamicMixedSpeakers, steps)
	if err != nil {
		dsinferWords.Close()
		dsinferParameters.Close()
		dsinferDynamicMixedSpeakers.Close()
		sendParameterError(ctx, events, err)
		return dsinfer.VarianceInferenceResult{}, false
	}

	return waitVarianceParameterInference(ctx, events, run, queued)
}

func newParameterInferenceInput(
	words []dsinfer.Word,
	parameters []dsinfer.Parameter,
	dynamicMixedSpeakers []dsinfer.DynamicMixedSpeaker,
) (*dsinfer.Words, *dsinfer.Parameters, *dsinfer.DynamicMixedSpeakers, error) {
	dsinferWords, err := dsinfer.NewWords(words)
	if err != nil {
		return nil, nil, nil, err
	}
	dsinferParameters, err := dsinfer.NewParameters(parameters)
	if err != nil {
		dsinferWords.Close()
		return nil, nil, nil, err
	}
	dsinferDynamicMixedSpeakers, err := dsinfer.NewDynamicMixedSpeakers(dynamicMixedSpeakers)
	if err != nil {
		dsinferWords.Close()
		dsinferParameters.Close()
		return nil, nil, nil, err
	}
	return dsinferWords, dsinferParameters, dsinferDynamicMixedSpeakers, nil
}

func waitPitchParameterInference(
	ctx context.Context,
	events chan<- api.ParameterEvent,
	run *dsinfer.PitchInferenceRun,
	queued *bool,
) (dsinfer.PitchInferenceResult, bool) {
	if !*queued {
		if !sendParameterEvent(ctx, events, api.ParameterEvent{State: api.StateQueuing}) {
			run.Terminate()
			<-run.Done()
			return dsinfer.PitchInferenceResult{}, false
		}
		*queued = true
	}

	started := run.Started()
	done := run.Done()
	for started != nil || done != nil {
		if started != nil {
			select {
			case <-started:
				started = nil
				if !sendParameterEvent(ctx, events, api.ParameterEvent{State: api.StateProcessing}) {
					run.Terminate()
					<-done
					return dsinfer.PitchInferenceResult{}, false
				}
				continue
			default:
			}
		}

		select {
		case <-ctx.Done():
			run.Terminate()
			<-done
			return dsinfer.PitchInferenceResult{}, false
		case <-started:
			started = nil
			if !sendParameterEvent(ctx, events, api.ParameterEvent{State: api.StateProcessing}) {
				run.Terminate()
				<-done
				return dsinfer.PitchInferenceResult{}, false
			}
		case result := <-done:
			return result, true
		}
	}
	return dsinfer.PitchInferenceResult{
		Err: api.NewError(api.ProblemTypeInternalError, "pitch inference stream ended without terminal state"),
	}, true
}

func waitVarianceParameterInference(
	ctx context.Context,
	events chan<- api.ParameterEvent,
	run *dsinfer.VarianceInferenceRun,
	queued *bool,
) (dsinfer.VarianceInferenceResult, bool) {
	if !*queued {
		if !sendParameterEvent(ctx, events, api.ParameterEvent{State: api.StateQueuing}) {
			run.Terminate()
			<-run.Done()
			return dsinfer.VarianceInferenceResult{}, false
		}
		*queued = true
	}

	started := run.Started()
	done := run.Done()
	for started != nil || done != nil {
		if started != nil {
			select {
			case <-started:
				started = nil
				if !sendParameterEvent(ctx, events, api.ParameterEvent{State: api.StateProcessing}) {
					run.Terminate()
					<-done
					return dsinfer.VarianceInferenceResult{}, false
				}
				continue
			default:
			}
		}

		select {
		case <-ctx.Done():
			run.Terminate()
			<-done
			return dsinfer.VarianceInferenceResult{}, false
		case <-started:
			started = nil
			if !sendParameterEvent(ctx, events, api.ParameterEvent{State: api.StateProcessing}) {
				run.Terminate()
				<-done
				return dsinfer.VarianceInferenceResult{}, false
			}
		case result := <-done:
			return result, true
		}
	}
	return dsinfer.VarianceInferenceResult{
		Err: api.NewError(api.ProblemTypeInternalError, "variance inference stream ended without terminal state"),
	}, true
}

func parseVarianceParameters(
	parameters []dsinfer.Parameter,
	requested map[string]bool,
	inputParameters map[string]api.Parameter,
) map[string]api.ParameterOutputParameter {
	output := make(map[string]api.ParameterOutputParameter, len(requested))
	seen := make(map[string]bool, len(varianceParameterIDs))
	for _, parameter := range parameters {
		id, values, sampleRate, err := builder.ParseParameter(parameter)
		if err != nil {
			panic(err)
		}
		if !isVarianceParameterID(id) {
			panic(fmt.Sprintf("unexpected variance parameter %q", id))
		}
		if seen[id] {
			panic(fmt.Sprintf("duplicate variance parameter %q", id))
		}
		seen[id] = true
		if requested[id] {
			output[id] = api.ParameterOutputParameter{
				Values:     values,
				SampleRate: sampleRate,
			}
		}
	}
	for _, id := range varianceParameterIDs {
		if !seen[id] && requested[id] {
			output[id] = api.ParameterOutputParameter{
				Values:     []float64{0},
				SampleRate: inputParameters[id].SampleRate,
			}
		}
	}
	return output
}

func isVarianceParameterID(id string) bool {
	_, ok := varianceParameterTagsByID[id]
	return ok
}

func makeEmptyParameterEvents() <-chan api.ParameterEvent {
	events := make(chan api.ParameterEvent, 1)
	events <- api.ParameterEvent{
		State: api.StateComplete,
		Output: api.ParameterOutput{
			Parameters: map[string]api.ParameterOutputParameter{},
		},
	}
	close(events)
	return events
}

func sendParameterEvent(ctx context.Context, events chan<- api.ParameterEvent, event api.ParameterEvent) bool {
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

func sendParameterError(ctx context.Context, events chan<- api.ParameterEvent, err error) {
	if err == nil || ctx.Err() != nil {
		return
	}
	sendParameterEvent(ctx, events, api.ParameterEvent{
		State: api.StateError,
		Err:   parameterAPIError(err),
	})
}

func cloneParameterOutput(parameters map[string]api.ParameterOutputParameter) map[string]api.ParameterOutputParameter {
	result := make(map[string]api.ParameterOutputParameter, len(parameters))
	for id, item := range parameters {
		result[id] = api.ParameterOutputParameter{
			Values:     append([]float64(nil), item.Values...),
			SampleRate: item.SampleRate,
		}
	}
	return result
}

func newInvalidParameterError(parameterID string, errorType string, detail string) error {
	return api.NewInvalidParameterError(parameterID, errorType, detail)
}

func parameterAPIError(err error) error {
	if err == nil {
		return nil
	}
	var apiError *api.Error
	if errors.As(err, &apiError) {
		return err
	}
	return api.NewError(api.ProblemTypeInternalError, err.Error())
}
