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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"

	"diffscope-synthesis-platform/internal/api"
	"diffscope-synthesis-platform/internal/dsinfer"
	"diffscope-synthesis-platform/internal/dsinfer/builder"
	"diffscope-synthesis-platform/internal/utils"
)

const (
	parameterIDGender    = "gender"
	parameterIDVelocity  = "velocity"
	parameterIDToneShift = "tone_shift"
)

var (
	audioParameterIDs = []string{
		parameterIDPitch,
		parameterIDBreathiness,
		parameterIDTension,
		parameterIDVoicing,
		parameterIDEnergy,
		parameterIDMouthOpening,
		parameterIDGender,
		parameterIDVelocity,
		parameterIDToneShift,
	}

	audioAcousticInferenceTaskMu       sync.Mutex
	audioAcousticInferenceTaskResource = utils.NewResourceManager[*dsinfer.AcousticInference, *dsinfer.AcousticInferenceTask](
		0,
		0,
		func(_ *dsinfer.AcousticInference, value *dsinfer.AcousticInferenceTask) {
			value.Delete()
		},
	)

	audioVocoderInferenceTaskMu       sync.Mutex
	audioVocoderInferenceTaskResource = utils.NewResourceManager[*dsinfer.VocoderInference, *dsinfer.VocoderInferenceTask](
		0,
		0,
		func(_ *dsinfer.VocoderInference, value *dsinfer.VocoderInferenceTask) {
			value.Delete()
		},
	)
)

func (Architecture) Audio(
	ctx context.Context,
	archExtra json.RawMessage,
	singers []api.Singer,
	mix [][]float64,
	mixSampleRate float64,
	pieceDuration float64,
	notes []api.Note,
	parameters map[string]api.AudioParameter,
) (<-chan api.AudioEvent, error) {
	extra, err := parseArchExtra(archExtra)
	if err != nil {
		return nil, err
	}

	acousticInference, vocoderInference, speakerIDs, err := prepareAudioSingers(singers)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	words, err := builder.BuildWords(speakerIDs, mix, mixSampleRate, convertParameterNotes(notes))
	if err != nil {
		return nil, audioAPIError(err)
	}
	dynamicMixedSpeakers, err := builder.BuildDynamicMixedSpeakers(speakerIDs, mixSampleRate, mix)
	if err != nil {
		return nil, audioAPIError(err)
	}
	audioParameters, err := buildAudioParameters(parameters)
	if err != nil {
		return nil, err
	}

	events := make(chan api.AudioEvent, 4)
	go runAudioInference(
		ctx,
		events,
		acousticInference,
		vocoderInference,
		pieceDuration,
		words,
		audioParameters,
		dynamicMixedSpeakers,
		extra.Depth,
		extra.Steps,
		getAudioFormat(),
	)
	return events, nil
}

func configureAudioResourceManagers() {
	audioAcousticInferenceTaskResource.SetTimeout(getInferenceCleanupTimeout())
	audioAcousticInferenceTaskResource.SetScanInterval(getInferenceCleanupInterval())
	audioVocoderInferenceTaskResource.SetTimeout(getInferenceCleanupTimeout())
	audioVocoderInferenceTaskResource.SetScanInterval(getInferenceCleanupInterval())
}

func prepareAudioSingers(singers []api.Singer) (*dsinfer.AcousticInference, *dsinfer.VocoderInference, []string, error) {
	var acousticInference *dsinfer.AcousticInference
	var acousticInferenceHandle uintptr
	var acousticInferenceSinger string
	var vocoderInference *dsinfer.VocoderInference
	var vocoderInferenceHandle uintptr
	var vocoderInferenceSinger string
	speakerIDs := make([]string, 0, len(singers))

	for _, singer := range singers {
		metadata, ok := getSingerMetadata(singer)
		if !ok {
			return nil, nil, nil, api.NewSingerNotExistError(singer.ID, "")
		}
		if metadata.acousticInference == nil || metadata.acousticInference.Handle() == 0 {
			return nil, nil, nil, api.NewSingerNotExistError(singer.ID, "")
		}
		if metadata.vocoderInference == nil || metadata.vocoderInference.Handle() == 0 {
			return nil, nil, nil, api.NewSingerNotExistError(singer.ID, "")
		}

		extra, err := parseSingerExtra(singer.Extra)
		if err != nil {
			return nil, nil, nil, err
		}
		if err := validateSingerExtraSpeaker(metadata, extra.Speaker); err != nil {
			return nil, nil, nil, err
		}

		currentAcousticHandle := metadata.acousticInference.Handle()
		if acousticInference == nil {
			acousticInference = metadata.acousticInference
			acousticInferenceHandle = currentAcousticHandle
			acousticInferenceSinger = singer.ID
		} else if currentAcousticHandle != acousticInferenceHandle {
			return nil, nil, nil, api.NewSingersUnmixableError(
				acousticInferenceSinger,
				singer.ID,
				"singers use different acoustic inference",
			)
		}

		currentVocoderHandle := metadata.vocoderInference.Handle()
		if vocoderInference == nil {
			vocoderInference = metadata.vocoderInference
			vocoderInferenceHandle = currentVocoderHandle
			vocoderInferenceSinger = singer.ID
		} else if currentVocoderHandle != vocoderInferenceHandle {
			return nil, nil, nil, api.NewSingersUnmixableError(
				vocoderInferenceSinger,
				singer.ID,
				"singers use different vocoder inference",
			)
		}

		speakerID, err := dsinfer.GetAcousticInferenceSpeakerID(metadata.SynthRTSinger, extra.Speaker)
		if err != nil {
			return nil, nil, nil, audioAPIError(err)
		}
		speakerIDs = append(speakerIDs, speakerID)
	}

	return acousticInference, vocoderInference, speakerIDs, nil
}

func buildAudioParameters(parameters map[string]api.AudioParameter) ([]dsinfer.Parameter, error) {
	result := make([]dsinfer.Parameter, 0, len(audioParameterIDs))
	for _, id := range audioParameterIDs {
		parameter, ok := parameters[id]
		if !ok {
			return nil, api.NewInvalidParameterError(id, "missing", fmt.Sprintf("missing %s parameter", id))
		}
		built, err := builder.BuildParameter(
			id,
			parameter.SampleRate,
			audioParameterValuesOrDefault(parameter.Values),
			false,
			0,
			0,
		)
		if err != nil {
			return nil, api.NewInvalidParameterError(id, "invalid_value", err.Error())
		}
		result = append(result, built)
	}
	return result, nil
}

func audioParameterValuesOrDefault(values []float64) []float64 {
	if len(values) > 0 {
		return values
	}
	return []float64{0}
}

func runAudioInference(
	ctx context.Context,
	events chan<- api.AudioEvent,
	acousticInference *dsinfer.AcousticInference,
	vocoderInference *dsinfer.VocoderInference,
	pieceDuration float64,
	words []dsinfer.Word,
	parameters []dsinfer.Parameter,
	dynamicMixedSpeakers []dsinfer.DynamicMixedSpeaker,
	depth float32,
	steps int64,
	format AudioFormat,
) {
	defer close(events)

	queued := false
	processing := false
	acousticResult, ok := runAcousticAudioInference(
		ctx,
		events,
		acousticInference,
		pieceDuration,
		words,
		parameters,
		dynamicMixedSpeakers,
		depth,
		steps,
		&queued,
		&processing,
	)
	if !ok {
		return
	}
	if acousticResult.Err != nil {
		sendAudioError(ctx, events, acousticResult.Err)
		return
	}
	feature := acousticResult.Feature
	defer feature.Close()

	vocoderResult, ok := runVocoderAudioInference(
		ctx,
		events,
		vocoderInference,
		feature,
		&queued,
		&processing,
	)
	if !ok {
		return
	}
	if vocoderResult.Err != nil {
		sendAudioError(ctx, events, vocoderResult.Err)
		return
	}
	audioData := vocoderResult.AudioData
	defer audioData.Close()

	if err := ctx.Err(); err != nil {
		return
	}
	data, err := encodeAudioData(audioData, format)
	if err != nil {
		sendAudioError(ctx, events, err)
		return
	}
	audioURL, err := makeAudioDataURL(format, data)
	if err != nil {
		sendAudioError(ctx, events, err)
		return
	}
	sendAudioEvent(ctx, events, api.AudioEvent{
		State: api.StateComplete,
		Output: api.AudioOutput{
			AudioURL: audioURL,
		},
	})
}

func runAcousticAudioInference(
	ctx context.Context,
	events chan<- api.AudioEvent,
	acousticInference *dsinfer.AcousticInference,
	pieceDuration float64,
	words []dsinfer.Word,
	parameters []dsinfer.Parameter,
	dynamicMixedSpeakers []dsinfer.DynamicMixedSpeaker,
	depth float32,
	steps int64,
	queued *bool,
	processing *bool,
) (dsinfer.AcousticInferenceResult, bool) {
	lease, err := acquireResource(&audioAcousticInferenceTaskMu, audioAcousticInferenceTaskResource, acousticInference, acousticInference.CreateTask)
	if err != nil {
		sendAudioError(ctx, events, err)
		return dsinfer.AcousticInferenceResult{}, false
	}
	defer lease.Release()

	dsinferWords, dsinferParameters, dsinferDynamicMixedSpeakers, err := newParameterInferenceInput(
		words,
		parameters,
		dynamicMixedSpeakers,
	)
	if err != nil {
		sendAudioError(ctx, events, err)
		return dsinfer.AcousticInferenceResult{}, false
	}

	run, err := lease.Value().Start(ctx, pieceDuration, dsinferWords, dsinferParameters, dsinferDynamicMixedSpeakers, depth, steps)
	if err != nil {
		dsinferWords.Close()
		dsinferParameters.Close()
		dsinferDynamicMixedSpeakers.Close()
		sendAudioError(ctx, events, err)
		return dsinfer.AcousticInferenceResult{}, false
	}

	return waitAcousticAudioInference(ctx, events, run, queued, processing)
}

func runVocoderAudioInference(
	ctx context.Context,
	events chan<- api.AudioEvent,
	vocoderInference *dsinfer.VocoderInference,
	feature *dsinfer.AcousticFeature,
	queued *bool,
	processing *bool,
) (dsinfer.VocoderInferenceResult, bool) {
	lease, err := acquireResource(&audioVocoderInferenceTaskMu, audioVocoderInferenceTaskResource, vocoderInference, vocoderInference.CreateTask)
	if err != nil {
		sendAudioError(ctx, events, err)
		return dsinfer.VocoderInferenceResult{}, false
	}
	defer lease.Release()

	run, err := lease.Value().Start(ctx, feature)
	if err != nil {
		sendAudioError(ctx, events, err)
		return dsinfer.VocoderInferenceResult{}, false
	}

	return waitVocoderAudioInference(ctx, events, run, queued, processing)
}

func waitAcousticAudioInference(
	ctx context.Context,
	events chan<- api.AudioEvent,
	run *dsinfer.AcousticInferenceRun,
	queued *bool,
	processing *bool,
) (dsinfer.AcousticInferenceResult, bool) {
	if !sendAudioQueued(ctx, events, run, run.Done(), queued) {
		return dsinfer.AcousticInferenceResult{}, false
	}

	started := run.Started()
	done := run.Done()
	for started != nil || done != nil {
		if started != nil {
			select {
			case <-started:
				started = nil
				if !sendAudioProcessing(ctx, events, run, done, processing) {
					return dsinfer.AcousticInferenceResult{}, false
				}
				continue
			default:
			}
		}

		select {
		case <-ctx.Done():
			run.Terminate()
			<-done
			return dsinfer.AcousticInferenceResult{}, false
		case <-started:
			started = nil
			if !sendAudioProcessing(ctx, events, run, done, processing) {
				return dsinfer.AcousticInferenceResult{}, false
			}
		case result := <-done:
			return result, true
		}
	}
	return dsinfer.AcousticInferenceResult{
		Err: api.NewError(api.ProblemTypeInternalError, "acoustic inference stream ended without terminal state"),
	}, true
}

func waitVocoderAudioInference(
	ctx context.Context,
	events chan<- api.AudioEvent,
	run *dsinfer.VocoderInferenceRun,
	queued *bool,
	processing *bool,
) (dsinfer.VocoderInferenceResult, bool) {
	if !sendAudioQueued(ctx, events, run, run.Done(), queued) {
		return dsinfer.VocoderInferenceResult{}, false
	}

	started := run.Started()
	done := run.Done()
	for started != nil || done != nil {
		if started != nil {
			select {
			case <-started:
				started = nil
				if !sendAudioProcessing(ctx, events, run, done, processing) {
					return dsinfer.VocoderInferenceResult{}, false
				}
				continue
			default:
			}
		}

		select {
		case <-ctx.Done():
			run.Terminate()
			<-done
			return dsinfer.VocoderInferenceResult{}, false
		case <-started:
			started = nil
			if !sendAudioProcessing(ctx, events, run, done, processing) {
				return dsinfer.VocoderInferenceResult{}, false
			}
		case result := <-done:
			return result, true
		}
	}
	return dsinfer.VocoderInferenceResult{
		Err: api.NewError(api.ProblemTypeInternalError, "vocoder inference stream ended without terminal state"),
	}, true
}

type audioInferenceRun interface {
	Terminate()
}

func sendAudioQueued[T any](
	ctx context.Context,
	events chan<- api.AudioEvent,
	run audioInferenceRun,
	done <-chan T,
	queued *bool,
) bool {
	if *queued {
		return true
	}
	if !sendAudioEvent(ctx, events, api.AudioEvent{State: api.StateQueuing}) {
		run.Terminate()
		<-done
		return false
	}
	*queued = true
	return true
}

func sendAudioProcessing[T any](
	ctx context.Context,
	events chan<- api.AudioEvent,
	run audioInferenceRun,
	done <-chan T,
	processing *bool,
) bool {
	if *processing {
		return true
	}
	if !sendAudioEvent(ctx, events, api.AudioEvent{State: api.StateProcessing}) {
		run.Terminate()
		<-done
		return false
	}
	*processing = true
	return true
}

func encodeAudioData(audioData *dsinfer.AudioData, format AudioFormat) ([]byte, error) {
	switch format {
	case AudioFormatWAV:
		return dsinfer.EncodeWAV(audioData)
	case AudioFormatFLAC:
		return dsinfer.EncodeFLAC(audioData)
	default:
		return nil, api.NewError(api.ProblemTypeInternalError, fmt.Sprintf("unsupported audio format %q", format))
	}
}

func makeAudioDataURL(format AudioFormat, data []byte) (string, error) {
	var mime string
	switch format {
	case AudioFormatWAV:
		mime = "audio/wav"
	case AudioFormatFLAC:
		mime = "audio/flac"
	default:
		return "", api.NewError(api.ProblemTypeInternalError, fmt.Sprintf("unsupported audio format %q", format))
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func sendAudioEvent(ctx context.Context, events chan<- api.AudioEvent, event api.AudioEvent) bool {
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

func sendAudioError(ctx context.Context, events chan<- api.AudioEvent, err error) {
	if err == nil || ctx.Err() != nil {
		return
	}
	sendAudioEvent(ctx, events, api.AudioEvent{
		State: api.StateError,
		Err:   audioAPIError(err),
	})
}

func audioAPIError(err error) error {
	return parameterAPIError(err)
}
