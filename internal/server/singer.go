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
	"net/http"

	"diffscope-synthesis-platform/internal/api"

	"github.com/gin-gonic/gin"
)

type singerInfoResponse struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Arch             string   `json:"arch"`
	MixGroup         string   `json:"mix_group"`
	Languages        []string `json:"languages"`
	DefaultLanguage  string   `json:"default_language"`
	ArchSpecificInfo any      `json:"arch_specific_info"`
	DefaultExtra     any      `json:"default_extra"`
}

type singerAvatarResponse struct {
	AvatarURL string `json:"avatar_url"`
}

type singerBackgroundResponse struct {
	BackgroundURL string `json:"background_url"`
}

func GetSingerList(c *gin.Context) {
	displayLanguage := c.Query("display_language")
	response := make([]singerInfoResponse, 0)
	for _, name := range registeredArchitectureNames() {
		arch, ok := getArchitecture(name)
		if !ok {
			writeSingerError(c, newUnknownArchError(name), problemContext{Arch: name})
			return
		}
		singers, err := arch.GetSingerList(displayLanguage)
		if err != nil {
			writeSingerError(c, err, problemContext{Arch: name})
			return
		}
		for _, singer := range singers {
			response = append(response, newSingerInfoResponse(name, singer))
		}
	}
	c.JSON(http.StatusOK, response)
}

func GetArchSingerList(c *gin.Context) {
	archName := c.Param("arch_id")
	arch, ok := getArchitecture(archName)
	if !ok {
		writeSingerError(c, newUnknownArchError(archName), problemContext{Arch: archName})
		return
	}
	singers, err := arch.GetSingerList(c.Query("display_language"))
	if err != nil {
		writeSingerError(c, err, problemContext{Arch: archName})
		return
	}
	response := make([]singerInfoResponse, 0, len(singers))
	for _, singer := range singers {
		response = append(response, newSingerInfoResponse(archName, singer))
	}
	c.JSON(http.StatusOK, response)
}

func GetArchSinger(c *gin.Context) {
	archName := c.Param("arch_id")
	singerID := c.Param("singer_id")
	arch, ok := getArchitecture(archName)
	if !ok {
		writeSingerError(c, newUnknownArchError(archName), problemContext{Arch: archName, Singer: singerID})
		return
	}
	singer, err := arch.GetSinger(singerID, c.Query("display_language"))
	if err != nil {
		writeSingerError(c, err, problemContext{Arch: archName, Singer: singerID})
		return
	}
	c.JSON(http.StatusOK, newSingerInfoResponse(archName, singer))
}

func GetArchSingerAvatar(c *gin.Context) {
	archName := c.Param("arch_id")
	singerID := c.Param("singer_id")
	context := problemContext{Arch: archName, Singer: singerID}
	arch, ok := getArchitecture(archName)
	if !ok {
		writeSingerError(c, newUnknownArchError(archName), context)
		return
	}
	avatarURL, err := arch.GetSingerAvatar(singerID, c.Query("display_language"))
	if err != nil {
		writeSingerError(c, err, context)
		return
	}
	c.JSON(http.StatusOK, singerAvatarResponse{AvatarURL: avatarURL})
}

func GetArchSingerBackground(c *gin.Context) {
	archName := c.Param("arch_id")
	singerID := c.Param("singer_id")
	context := problemContext{Arch: archName, Singer: singerID}
	arch, ok := getArchitecture(archName)
	if !ok {
		writeSingerError(c, newUnknownArchError(archName), context)
		return
	}
	backgroundURL, err := arch.GetSingerBackground(singerID, c.Query("display_language"))
	if err != nil {
		writeSingerError(c, err, context)
		return
	}
	c.JSON(http.StatusOK, singerBackgroundResponse{BackgroundURL: backgroundURL})
}

func GetArchSingerDemoAudioList(c *gin.Context) {
	archName := c.Param("arch_id")
	singerID := c.Param("singer_id")
	context := problemContext{Arch: archName, Singer: singerID}
	arch, ok := getArchitecture(archName)
	if !ok {
		writeSingerError(c, newUnknownArchError(archName), context)
		return
	}
	demoAudio, err := arch.GetSingerDemoAudioList(singerID, c.Query("display_language"))
	if err != nil {
		writeSingerError(c, err, context)
		return
	}
	c.JSON(http.StatusOK, demoAudio)
}

func newSingerInfoResponse(arch string, singer api.SingerInfo) singerInfoResponse {
	return singerInfoResponse{
		ID:               singer.ID,
		Name:             singer.Name,
		Arch:             arch,
		MixGroup:         singer.MixGroup,
		Languages:        singer.Languages,
		DefaultLanguage:  singer.DefaultLanguage,
		ArchSpecificInfo: singer.ArchSpecificInfo,
		DefaultExtra:     singer.DefaultExtra,
	}
}

func writeSingerError(c *gin.Context, err error, context problemContext) {
	writeProblem(c, err, context)
}
