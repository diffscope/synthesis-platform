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

	"github.com/gin-gonic/gin"
)

var envTagLogger = slog.With("component", "server.env_tag")

type envTagRequest struct {
	Context *multiSingerContext `json:"context" validate:"required"`
}

type envTagResponse struct {
	EnvTag string `json:"env_tag"`
}

func PostEnvTag(c *gin.Context) {
	var request envTagRequest
	if err := decodeRequest(c, &request); err != nil {
		envTagLogger.Error("Invalid environment tag request", slog.Any("error", err))
		writeBadRequest(c, err)
		return
	}

	archName := *request.Context.Arch
	context := problemContext{Arch: archName, Singers: singerIDs(request.Context.Singers)}
	arch, ok := getArchitecture(archName)
	if !ok {
		writeProblem(c, newUnknownArchError(archName), context)
		return
	}

	c.JSON(http.StatusOK, envTagResponse{
		EnvTag: arch.GetEnvTag(*request.Context.ArchExtra, request.Context.singers()),
	})
}
