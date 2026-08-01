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

var parameterLogger = slog.With("component", "server.parameter")

type parameterRequest struct {
	Context *multiSingerContext        `json:"context" validate:"required"`
	Input   *api.ParameterInputRequest `json:"input" validate:"required"`
	Stream  *bool                      `json:"stream"`
}

type parameterResponse struct {
	State  api.State           `json:"state"`
	Output api.ParameterOutput `json:"output"`
}

type parameterOutputResponse struct {
	State  api.State           `json:"state"`
	Output api.ParameterOutput `json:"output"`
}

type parameterStateResponse struct {
	State api.State `json:"state"`
}

func PostParameter(c *gin.Context) {
	var request parameterRequest
	if err := decodeRequest(c, &request); err != nil {
		parameterLogger.Error("Invalid parameter request", slog.Any("error", err))
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

	input := request.Input.ToParameterInput()
	events, err := arch.Parameter(
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
		writeProblem(c, api.NewError(api.ProblemTypeInternalError, "parameter stream is nil"), context)
		return
	}

	if request.stream() {
		writeParameterStream(c, events, context)
		return
	}
	writeParameterResponse(c, events, context)
}

func (r parameterRequest) stream() bool {
	return r.Stream != nil && *r.Stream
}

func writeParameterResponse(c *gin.Context, events <-chan api.ParameterEvent, context problemContext) {
	output, err := readParameterEvents(events)
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
	c.JSON(http.StatusOK, parameterResponse{
		State:  api.StateComplete,
		Output: output,
	})
}

func readParameterEvents(events <-chan api.ParameterEvent) (api.ParameterOutput, error) {
	output := emptyParameterOutput()
	var previous api.State
	for event := range events {
		if err := validateParameterTransition(previous, event.State); err != nil {
			return api.ParameterOutput{}, err
		}
		switch event.State {
		case api.StateComplete:
			mergeParameterOutput(&output, event.Output)
			return output, nil
		case api.StateError:
			if event.Err != nil {
				return api.ParameterOutput{}, event.Err
			}
			return api.ParameterOutput{}, api.NewError(api.ProblemTypeInternalError, "")
		case api.StateQueuing:
			previous = event.State
		case api.StateProcessing:
			mergeParameterOutput(&output, event.Output)
			previous = event.State
		default:
			return api.ParameterOutput{}, invalidParameterStateError()
		}
	}
	return api.ParameterOutput{}, api.NewError(api.ProblemTypeInternalError, "parameter stream ended without terminal state")
}

func writeParameterStream(c *gin.Context, events <-chan api.ParameterEvent, context problemContext) {
	writer := c.Writer
	writer.Header().Set("Content-Type", "application/x-ndjson")
	writer.WriteHeader(http.StatusOK)

	encoder := json.NewEncoder(writer)
	var previous api.State
	for event := range events {
		if c.Request.Context().Err() != nil {
			return
		}
		if err := validateParameterTransition(previous, event.State); err != nil {
			writeParameterStreamError(encoder, writer, err, context)
			return
		}
		switch event.State {
		case api.StateQueuing:
			if err := encoder.Encode(parameterStateResponse{State: event.State}); err != nil {
				return
			}
			previous = event.State
		case api.StateProcessing:
			if event.Output.Parameters == nil {
				if err := encoder.Encode(parameterStateResponse{State: event.State}); err != nil {
					return
				}
			} else if err := encoder.Encode(parameterOutputResponse{
				State:  event.State,
				Output: event.Output,
			}); err != nil {
				return
			}
			previous = event.State
		case api.StateComplete:
			if err := encoder.Encode(parameterResponse{
				State:  api.StateComplete,
				Output: ensureParameterOutput(event.Output),
			}); err != nil {
				return
			}
			flushParameterStream(writer)
			return
		case api.StateError:
			err := event.Err
			if err == nil {
				err = api.NewError(api.ProblemTypeInternalError, "")
			}
			writeParameterStreamError(encoder, writer, err, context)
			return
		default:
			writeParameterStreamError(encoder, writer, invalidParameterStateError(), context)
			return
		}
		flushParameterStream(writer)
	}
	writeParameterStreamError(
		encoder,
		writer,
		api.NewError(api.ProblemTypeInternalError, "parameter stream ended without terminal state"),
		context,
	)
}

func writeParameterStreamError(encoder *json.Encoder, writer gin.ResponseWriter, err error, context problemContext) {
	_ = encoder.Encode(newStreamProblem(err, context))
	flushParameterStream(writer)
}

func validateParameterTransition(previous api.State, current api.State) error {
	switch current {
	case api.StateComplete, api.StateError:
		return nil
	case api.StateQueuing:
		if previous == "" {
			return nil
		}
	case api.StateProcessing:
		if previous == "" || previous == api.StateQueuing || previous == api.StateProcessing {
			return nil
		}
	}
	return invalidParameterStateError()
}

func invalidParameterStateError() error {
	return api.NewError(api.ProblemTypeInternalError, "invalid parameter state transition")
}

func emptyParameterOutput() api.ParameterOutput {
	return api.ParameterOutput{
		Parameters: map[string]api.ParameterOutputParameter{},
	}
}

func ensureParameterOutput(output api.ParameterOutput) api.ParameterOutput {
	if output.Parameters == nil {
		return emptyParameterOutput()
	}
	return output
}

func mergeParameterOutput(target *api.ParameterOutput, source api.ParameterOutput) {
	if source.Parameters == nil {
		return
	}
	if target.Parameters == nil {
		target.Parameters = make(map[string]api.ParameterOutputParameter, len(source.Parameters))
	}
	for name, parameter := range source.Parameters {
		target.Parameters[name] = api.ParameterOutputParameter{
			Values:     append([]float64(nil), parameter.Values...),
			SampleRate: parameter.SampleRate,
		}
	}
}

func flushParameterStream(writer gin.ResponseWriter) {
	if flusher, ok := any(writer).(http.Flusher); ok {
		flusher.Flush()
	}
}
