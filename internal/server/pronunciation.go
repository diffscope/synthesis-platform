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

var logger = slog.With("component", "server.pronunciation")

type pronunciationRequest struct {
	Context *pronunciationContext `json:"context" validate:"required"`
	Input   *pronunciationInput   `json:"input" validate:"required"`
}

type pronunciationContext struct {
	Arch      *string            `json:"arch" validate:"required"`
	ArchExtra *json.RawMessage   `json:"arch_extra" validate:"required"`
	Singer    *api.SingerRequest `json:"singer" validate:"required"`
}

type pronunciationInput struct {
	Notes []api.LyricRequest `json:"notes" validate:"required,dive"`
}

type pronunciationResponse struct {
	State  api.State           `json:"state"`
	Output pronunciationOutput `json:"output"`
}

type pronunciationOutput struct {
	Notes []api.Pronunciation `json:"notes"`
}

func PostPronunciation(c *gin.Context) {
	var request pronunciationRequest
	if err := decodeRequest(c, &request); err != nil {
		logger.Error("Invalid pronunciation request", slog.Any("error", err))
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

	pronunciations, err := arch.Pronunciation(
		c.Request.Context(),
		archExtra,
		singer,
		pronunciationLyrics(request.Input.Notes),
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
	c.JSON(http.StatusOK, pronunciationResponse{
		State: api.StateComplete,
		Output: pronunciationOutput{
			Notes: pronunciations,
		},
	})
}

func pronunciationLyrics(requests []api.LyricRequest) []api.Lyric {
	lyrics := make([]api.Lyric, 0, len(requests))
	for _, request := range requests {
		lyrics = append(lyrics, request.ToLyric())
	}
	return lyrics
}
