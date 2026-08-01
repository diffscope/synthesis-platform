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
	"errors"
	"log/slog"
	"net/http"

	"diffscope-synthesis-platform/internal/api"

	"github.com/gin-gonic/gin"
)

const problemJSONMediaType = "application/problem+json"

var problemLogger = slog.With("component", "server.problem")

type problemContext struct {
	Arch    string
	Singer  string
	Singers []string
}

type problemDetails struct {
	State     api.State             `json:"state"`
	Type      api.ProblemType       `json:"type"`
	Title     string                `json:"title"`
	Status    int                   `json:"status,omitempty"`
	Detail    string                `json:"detail"`
	Arch      string                `json:"arch,omitempty"`
	Singer    string                `json:"singer,omitempty"`
	Singers   []string              `json:"singers,omitempty"`
	Parameter *api.ParameterIssue   `json:"parameter,omitempty"`
	Errors    []api.ValidationIssue `json:"errors,omitempty"`
}

func writeProblem(c *gin.Context, err error, context problemContext) {
	if c.Request.Context().Err() != nil {
		return
	}

	apiError, ok := toAPIError(err)
	if !ok {
		problemLogger.Error("Internal error occurred", slog.Any("error", err))
	}
	status := problemHTTPStatus(apiError.Type)
	c.Header("Content-Type", problemJSONMediaType)
	c.JSON(status, newProblemDetails(apiError, status, context))
}

func newStreamProblem(err error, context problemContext) problemDetails {
	apiError, ok := toAPIError(err)
	if !ok {
		problemLogger.Error("Internal stream error occurred", slog.Any("error", err))
	}
	return newProblemDetails(apiError, 0, context)
}

func newProblemDetails(apiError *api.Error, status int, context problemContext) problemDetails {
	problem := problemDetails{
		State:  api.StateError,
		Type:   apiError.Type,
		Title:  problemTitle(apiError.Type),
		Status: status,
		Detail: apiError.Detail,
	}

	switch apiError.Type {
	case api.ProblemTypeUnknownArch:
		problem.Arch = firstNonEmpty(apiError.Arch, context.Arch)
	case api.ProblemTypeSingerNotExist:
		problem.Arch = context.Arch
		problem.Singer = firstNonEmpty(apiError.Singer, context.Singer)
		if problem.Singer == "" && len(context.Singers) == 1 {
			problem.Singer = context.Singers[0]
		}
	case api.ProblemTypeSingerConfigInvalid, api.ProblemTypeValidationError:
		problem.Errors = append([]api.ValidationIssue(nil), apiError.Errors...)
	case api.ProblemTypeInvalidParameter:
		if apiError.Parameter != nil {
			parameter := *apiError.Parameter
			problem.Parameter = &parameter
		}
	case api.ProblemTypeSingersUnmixable:
		problem.Arch = context.Arch
		problem.Singers = append([]string(nil), apiError.Singers...)
		if len(problem.Singers) == 0 && len(context.Singers) >= 2 {
			problem.Singers = append([]string(nil), context.Singers[:2]...)
		}
	}

	return problem
}

func toAPIError(err error) (*api.Error, bool) {
	var apiError *api.Error
	if errors.As(err, &apiError) {
		return apiError, true
	}
	detail := ""
	if err != nil {
		detail = err.Error()
	}
	return api.NewError(api.ProblemTypeInternalError, detail), false
}

func problemHTTPStatus(problemType api.ProblemType) int {
	switch problemType {
	case api.ProblemTypeUnknownArch, api.ProblemTypeSingerNotExist:
		return http.StatusNotFound
	case api.ProblemTypeSingerConfigInvalid, api.ProblemTypeInvalidParameter, api.ProblemTypeSingersUnmixable:
		return http.StatusUnprocessableEntity
	case api.ProblemTypeValidationError:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func problemTitle(problemType api.ProblemType) string {
	switch problemType {
	case api.ProblemTypeUnknownArch:
		return "Unknown arch"
	case api.ProblemTypeSingerNotExist:
		return "Singer not exist"
	case api.ProblemTypeSingerConfigInvalid:
		return "Singer config invalid"
	case api.ProblemTypeInvalidParameter:
		return "Invalid parameter"
	case api.ProblemTypeSingersUnmixable:
		return "Singers unmixable"
	case api.ProblemTypeValidationError:
		return "Validation error"
	default:
		return "Internal error"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
