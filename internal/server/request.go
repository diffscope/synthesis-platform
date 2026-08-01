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
	"errors"
	"io"
	"reflect"
	"strings"

	"diffscope-synthesis-platform/internal/api"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

var requestValidator = newRequestValidator()

type multiSingerContext struct {
	Arch      *string             `json:"arch" validate:"required"`
	ArchExtra *json.RawMessage    `json:"arch_extra" validate:"required"`
	Singers   []api.SingerRequest `json:"singers" validate:"required,min=1,dive"`
}

func (c multiSingerContext) singers() []api.Singer {
	singers := make([]api.Singer, 0, len(c.Singers))
	for _, singer := range c.Singers {
		singers = append(singers, singer.ToSinger())
	}
	return singers
}

func singerIDs(singers []api.SingerRequest) []string {
	ids := make([]string, 0, len(singers))
	for _, singer := range singers {
		if singer.ID != nil {
			ids = append(ids, *singer.ID)
		}
	}
	return ids
}

func newRequestValidator() *validator.Validate {
	validate := validator.New()
	validate.RegisterTagNameFunc(func(field reflect.StructField) string {
		name := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]
		if name == "" || name == "-" {
			return field.Name
		}
		return name
	})
	validate.RegisterStructValidation(validateDurationRequest, durationRequest{})
	validate.RegisterStructValidation(validateParameterRequest, parameterRequest{})
	validate.RegisterStructValidation(validateAudioRequest, audioRequest{})
	return validate
}

func decodeRequest(c *gin.Context, value any) error {
	if err := decodeJSON(c, value); err != nil {
		return err
	}
	return requestValidator.Struct(value)
}

func decodeJSON(c *gin.Context, value any) error {
	decoder := json.NewDecoder(c.Request.Body)
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body must contain one JSON value")
	}
	return nil
}

func writeBadRequest(c *gin.Context, err error) {
	writeProblem(c, newRequestValidationError(err), problemContext{})
}

func newRequestValidationError(err error) error {
	detail := "The request body is invalid."
	issues := requestValidationIssues(err)
	return api.NewValidationError(detail, issues...)
}

func requestValidationIssues(err error) []api.ValidationIssue {
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		issues := make([]api.ValidationIssue, 0, len(validationErrors))
		for _, validationError := range validationErrors {
			pointer := validationPointer(validationError.Namespace())
			switch validationError.Tag() {
			case "duration_mix", "parameter_mix", "audio_mix":
				pointer = "#/input/mix"
			}
			issues = append(issues, api.ValidationIssue{
				Pointer: pointer,
				Type:    validationIssueType(validationError.Tag()),
				Detail:  validationIssueDetail(validationError),
			})
		}
		return issues
	}

	pointer := "#"
	issueType := "invalid_json"
	issueDetail := "must contain exactly one valid JSON value"

	var unmarshalTypeError *json.UnmarshalTypeError
	switch {
	case errors.As(err, &unmarshalTypeError):
		pointer = jsonFieldPointer(unmarshalTypeError.Field)
		issueType = "invalid_type"
		issueDetail = unmarshalTypeError.Error()
	case errors.Is(err, io.EOF):
		issueType = "required"
		issueDetail = "request body is required"
	case err != nil:
		issueDetail = err.Error()
	}

	return []api.ValidationIssue{{
		Pointer: pointer,
		Type:    issueType,
		Detail:  issueDetail,
	}}
}

func validationPointer(namespace string) string {
	parts := splitFieldPath(namespace)
	if len(parts) > 0 {
		parts = parts[1:]
	}
	return jsonPointer(parts)
}

func jsonFieldPointer(field string) string {
	return jsonPointer(splitFieldPath(field))
}

func splitFieldPath(field string) []string {
	field = strings.NewReplacer("[", ".", "]", "").Replace(field)
	rawParts := strings.Split(field, ".")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func jsonPointer(parts []string) string {
	if len(parts) == 0 {
		return "#"
	}
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.ReplaceAll(part, "~", "~0")
		part = strings.ReplaceAll(part, "/", "~1")
		escaped = append(escaped, part)
	}
	return "#/" + strings.Join(escaped, "/")
}

func validationIssueType(tag string) string {
	switch tag {
	case "duration_mix", "parameter_mix", "audio_mix":
		return "invalid_mix"
	default:
		return tag
	}
}

func validationIssueDetail(field validator.FieldError) string {
	switch field.Tag() {
	case "required":
		return "is required"
	case "min":
		return "must contain at least " + field.Param() + " item(s)"
	case "gte":
		return "must be greater than or equal to " + field.Param()
	case "lte":
		return "must be less than or equal to " + field.Param()
	case "gt":
		return "must be greater than " + field.Param()
	case "duration_mix", "parameter_mix", "audio_mix":
		return "must contain one value per additional singer, use values from 0 to 1, and have a sum no greater than 1"
	default:
		return "failed " + field.Tag() + " validation"
	}
}

func validateDurationRequest(sl validator.StructLevel) {
	var request durationRequest
	switch current := sl.Current().Interface().(type) {
	case durationRequest:
		request = current
	case *durationRequest:
		if current == nil {
			return
		}
		request = *current
	default:
		return
	}

	if request.Context == nil || request.Input == nil || request.Context.Singers == nil || request.Input.Mix == nil {
		return
	}

	validateMix(sl, request.Context.Singers, request.Input.Mix, "duration_mix")
}

func validateParameterRequest(sl validator.StructLevel) {
	var request parameterRequest
	switch current := sl.Current().Interface().(type) {
	case parameterRequest:
		request = current
	case *parameterRequest:
		if current == nil {
			return
		}
		request = *current
	default:
		return
	}

	if request.Context == nil || request.Input == nil || request.Context.Singers == nil || request.Input.Mix == nil {
		return
	}

	validateMix(sl, request.Context.Singers, request.Input.Mix, "parameter_mix")
}

func validateAudioRequest(sl validator.StructLevel) {
	var request audioRequest
	switch current := sl.Current().Interface().(type) {
	case audioRequest:
		request = current
	case *audioRequest:
		if current == nil {
			return
		}
		request = *current
	default:
		return
	}

	if request.Context == nil || request.Input == nil || request.Context.Singers == nil || request.Input.Mix == nil {
		return
	}

	validateMix(sl, request.Context.Singers, request.Input.Mix, "audio_mix")
}

func validateMix(sl validator.StructLevel, singers []api.SingerRequest, mixes [][]float64, tag string) {
	expectedLength := len(singers) - 1
	if expectedLength < 0 {
		return
	}
	for _, mix := range mixes {
		if !isValidMix(mix, expectedLength) {
			sl.ReportError(mixes, "Mix", "Mix", tag, "")
			return
		}
	}
}

func isValidMix(mix []float64, expectedLength int) bool {
	if mix == nil || len(mix) != expectedLength {
		return false
	}
	var sum float64
	for _, value := range mix {
		if value < 0 || value > 1 {
			return false
		}
		sum += value
	}
	return sum >= 0 && sum <= 1
}
