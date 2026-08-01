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

package api

type ProblemType string

const (
	ProblemTypeInternalError       ProblemType = "/problems/internal_error"
	ProblemTypeUnknownArch         ProblemType = "/problems/unknown_arch"
	ProblemTypeSingerNotExist      ProblemType = "/problems/singer_not_exist"
	ProblemTypeSingerConfigInvalid ProblemType = "/problems/singer_config_invalid"
	ProblemTypeInvalidParameter    ProblemType = "/problems/invalid_parameter"
	ProblemTypeSingersUnmixable    ProblemType = "/problems/singers_unmixable"
	ProblemTypeValidationError     ProblemType = "/problems/validation_error"
)

type ValidationIssue struct {
	Pointer string `json:"pointer"`
	Type    string `json:"type"`
	Detail  string `json:"detail"`
}

type ParameterIssue struct {
	ID        string `json:"id"`
	ErrorType string `json:"error_type"`
}

type Error struct {
	Type      ProblemType
	Detail    string
	Arch      string
	Singer    string
	Singers   []string
	Parameter *ParameterIssue
	Errors    []ValidationIssue
}

func NewError(problemType ProblemType, detail string) *Error {
	return &Error{
		Type:   problemType,
		Detail: detail,
	}
}

func NewUnknownArchError(arch string, detail string) *Error {
	err := NewError(ProblemTypeUnknownArch, detail)
	err.Arch = arch
	return err
}

func NewSingerNotExistError(singer string, detail string) *Error {
	err := NewError(ProblemTypeSingerNotExist, detail)
	err.Singer = singer
	return err
}

func NewSingerConfigInvalidError(detail string, issues ...ValidationIssue) *Error {
	err := NewError(ProblemTypeSingerConfigInvalid, detail)
	err.Errors = append([]ValidationIssue(nil), issues...)
	return err
}

func NewInvalidParameterError(parameterID string, errorType string, detail string) *Error {
	err := NewError(ProblemTypeInvalidParameter, detail)
	err.Parameter = &ParameterIssue{
		ID:        parameterID,
		ErrorType: errorType,
	}
	return err
}

func NewSingersUnmixableError(firstSinger string, secondSinger string, detail string) *Error {
	err := NewError(ProblemTypeSingersUnmixable, detail)
	err.Singers = []string{firstSinger, secondSinger}
	return err
}

func NewValidationError(detail string, issues ...ValidationIssue) *Error {
	err := NewError(ProblemTypeValidationError, detail)
	err.Errors = append([]ValidationIssue(nil), issues...)
	return err
}

func (e *Error) Error() string {
	if e.Detail != "" {
		return e.Detail
	}
	return string(e.Type)
}
