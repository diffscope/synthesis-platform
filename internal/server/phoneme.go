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
	"log/slog"
	"net/http"

	"diffscope-synthesis-platform/internal/api"

	"github.com/gin-gonic/gin"
)

var phonemeLogger = slog.With("component", "server.phoneme")

type phonemeRequest struct {
	Context *pronunciationContext `json:"context" validate:"required"`
	Input   *phonemeInput         `json:"input" validate:"required"`
}

type phonemeInput struct {
	Notes []api.PronunciationNoteRequest `json:"notes" validate:"required,dive"`
}

type phonemeResponse struct {
	State  api.State     `json:"state"`
	Output phonemeOutput `json:"output"`
}

type phonemeOutput struct {
	Notes []api.PhonemeNote `json:"notes"`
}

func PostPhoneme(c *gin.Context) {
	var request phonemeRequest
	if err := decodeRequest(c, &request); err != nil {
		phonemeLogger.Error("Invalid phoneme request", slog.Any("error", err))
		writeBadRequest(c, err)
		return
	}

	archExtra := *request.Context.ArchExtra
	singer := request.Context.Singer.ToSinger()
	archName := *request.Context.Arch
	context := problemContext{Arch: archName, Singer: singer.ID}
	arch, ok := getArchitecture(archName)
	if !ok {
		writeProblem(c, newUnknownArchError(archName), context)
		return
	}

	notes, err := arch.Phoneme(
		c.Request.Context(),
		archExtra,
		singer,
		phonemeNotes(request.Input.Notes),
	)
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
	c.JSON(http.StatusOK, phonemeResponse{
		State: api.StateComplete,
		Output: phonemeOutput{
			Notes: notes,
		},
	})
}

func phonemeNotes(requests []api.PronunciationNoteRequest) []api.PronunciationNote {
	notes := make([]api.PronunciationNote, 0, len(requests))
	for _, request := range requests {
		notes = append(notes, request.ToPronunciationNote())
	}
	return notes
}
