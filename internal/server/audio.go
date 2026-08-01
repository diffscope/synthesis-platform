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

package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"diffscope-synthesis-platform/internal/api"

	"github.com/gin-gonic/gin"
)

var audioLogger = slog.With("component", "server.audio")

type audioRequest struct {
	Context *multiSingerContext    `json:"context" validate:"required"`
	Input   *api.AudioInputRequest `json:"input" validate:"required"`
	Stream  *bool                  `json:"stream"`
}

type audioResponse struct {
	State  api.State       `json:"state"`
	Output api.AudioOutput `json:"output"`
}

type audioStateResponse struct {
	State api.State `json:"state"`
}

func PostAudio(c *gin.Context) {
	var request audioRequest
	if err := decodeRequest(c, &request); err != nil {
		audioLogger.Error("Invalid audio request", slog.Any("error", err))
		writeBadRequest(c, err)
		return
	}

	archExtra := *request.Context.ArchExtra
	archName := *request.Context.Arch
	context := problemContext{Arch: archName, Singers: singerIDs(request.Context.Singers)}
	arch, ok := getArchitecture(archName)
	if !ok {
		writeProblem(c, newUnknownArchError(archName), context)
		return
	}

	singers := request.Context.singers()

	input := request.Input.ToAudioInput()
	events, err := arch.Audio(
		c.Request.Context(),
		archExtra,
		singers,
		request.Input.Mix,
		*request.Input.MixSampleRate,
		input.PieceDuration,
		input.Notes,
		input.Parameters,
	)
	if err != nil {
		if c.Request.Context().Err() != nil {
			return
		}
		writeProblem(c, err, context)
		return
	}
	if events == nil {
		writeProblem(c, api.NewError(api.ProblemTypeInternalError, "audio stream is nil"), context)
		return
	}

	if request.stream() {
		writeAudioStream(c, events, context)
		return
	}
	writeAudioResponse(c, events, context)
}

func (r audioRequest) stream() bool {
	return r.Stream != nil && *r.Stream
}

func writeAudioResponse(c *gin.Context, events <-chan api.AudioEvent, context problemContext) {
	final, err := readAudioEvents(events)
	if err != nil {
		if c.Request.Context().Err() != nil {
			return
		}
		writeProblem(c, err, context)
		return
	}
	if c.Request.Context().Err() != nil {
		return
	}
	c.JSON(http.StatusOK, audioResponse{
		State:  api.StateComplete,
		Output: final.Output,
	})
}

func readAudioEvents(events <-chan api.AudioEvent) (api.AudioEvent, error) {
	var previous api.State
	for event := range events {
		if err := validateAudioTransition(previous, event.State); err != nil {
			return api.AudioEvent{}, err
		}
		switch event.State {
		case api.StateComplete:
			return event, nil
		case api.StateError:
			if event.Err != nil {
				return api.AudioEvent{}, event.Err
			}
			return api.AudioEvent{}, api.NewError(api.ProblemTypeInternalError, "")
		case api.StateQueuing, api.StateProcessing:
			previous = event.State
		default:
			return api.AudioEvent{}, invalidAudioStateError()
		}
	}
	return api.AudioEvent{}, api.NewError(api.ProblemTypeInternalError, "audio stream ended without terminal state")
}

func writeAudioStream(c *gin.Context, events <-chan api.AudioEvent, context problemContext) {
	writer := c.Writer
	writer.Header().Set("Content-Type", "application/x-ndjson")
	writer.WriteHeader(http.StatusOK)

	encoder := json.NewEncoder(writer)
	var previous api.State
	for event := range events {
		if c.Request.Context().Err() != nil {
			return
		}
		if err := validateAudioTransition(previous, event.State); err != nil {
			writeAudioStreamError(encoder, writer, err, context)
			return
		}
		switch event.State {
		case api.StateQueuing, api.StateProcessing:
			if err := encoder.Encode(audioStateResponse{State: event.State}); err != nil {
				return
			}
			previous = event.State
		case api.StateComplete:
			if err := encoder.Encode(audioResponse{
				State:  api.StateComplete,
				Output: event.Output,
			}); err != nil {
				return
			}
			flushAudioStream(writer)
			return
		case api.StateError:
			err := event.Err
			if err == nil {
				err = api.NewError(api.ProblemTypeInternalError, "")
			}
			writeAudioStreamError(encoder, writer, err, context)
			return
		default:
			writeAudioStreamError(encoder, writer, invalidAudioStateError(), context)
			return
		}
		flushAudioStream(writer)
	}
	writeAudioStreamError(
		encoder,
		writer,
		api.NewError(api.ProblemTypeInternalError, "audio stream ended without terminal state"),
		context,
	)
}

func writeAudioStreamError(encoder *json.Encoder, writer gin.ResponseWriter, err error, context problemContext) {
	_ = encoder.Encode(newStreamProblem(err, context))
	flushAudioStream(writer)
}

func validateAudioTransition(previous api.State, current api.State) error {
	switch current {
	case api.StateComplete, api.StateError:
		return nil
	case api.StateQueuing:
		if previous == "" {
			return nil
		}
	case api.StateProcessing:
		if previous == "" || previous == api.StateQueuing {
			return nil
		}
	}
	return invalidAudioStateError()
}

func invalidAudioStateError() error {
	return api.NewError(api.ProblemTypeInternalError, "invalid audio state transition")
}

func flushAudioStream(writer gin.ResponseWriter) {
	if flusher, ok := any(writer).(http.Flusher); ok {
		flusher.Flush()
	}
}
