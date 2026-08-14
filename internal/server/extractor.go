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

type extractorInfoResponse struct {
	ID                       string `json:"id"`
	Name                     string `json:"name"`
	PreferredAudioSampleRate int    `json:"preferred_audio_sample_rate"`
}

type extractorListResponse struct {
	Note  []extractorInfoResponse `json:"note"`
	Tempo []extractorInfoResponse `json:"tempo"`
	Pitch []extractorInfoResponse `json:"pitch"`
}

func GetExtractorList(c *gin.Context) {
	c.JSON(http.StatusOK, extractorListResponse{
		Note:  make([]extractorInfoResponse, 0),
		Tempo: make([]extractorInfoResponse, 0),
		Pitch: make([]extractorInfoResponse, 0),
	})
}

func GetNoteExtractor(c *gin.Context) {
	writeExtractionNotImplemented(c)
}

func GetTempoExtractor(c *gin.Context) {
	writeExtractionNotImplemented(c)
}

func GetPitchExtractor(c *gin.Context) {
	writeExtractionNotImplemented(c)
}

func PostExtractNote(c *gin.Context) {
	writeExtractionNotImplemented(c)
}

func PostExtractTempo(c *gin.Context) {
	writeExtractionNotImplemented(c)
}

func PostExtractPitch(c *gin.Context) {
	writeExtractionNotImplemented(c)
}

func writeExtractionNotImplemented(c *gin.Context) {
	writeProblem(c, api.NewError(api.ProblemTypeNotImplemented, "extraction is not implemented by this service"), problemContext{})
}
