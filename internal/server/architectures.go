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
	"strings"
	"sync"

	"diffscope-synthesis-platform/internal/api"
	"diffscope-synthesis-platform/internal/architecture"

	"github.com/gin-gonic/gin"
)

var (
	architecturesMu          sync.RWMutex
	registeredArchitectures  = make(map[string]architecture.Architecture)
	registeredArchitectureDB = architecture.NewRegistry(registeredArchitectures)
)

type architectureMetadataResponse struct {
	ID                string                                       `json:"id"`
	Name              string                                       `json:"name"`
	PronunciationMode api.ArchitectureSynthesisMode                `json:"pronunciation_mode"`
	PhonemeMode       api.ArchitectureSynthesisMode                `json:"phoneme_mode"`
	Parameters        map[string]api.ArchitectureParameterMetadata `json:"parameters"`
	AudioDependencies []string                                     `json:"audio_dependencies"`
}

func RegisterArchitecture(name string, implementation architecture.Architecture) {
	if name == "" {
		panic("server: empty architecture name")
	}
	if implementation == nil {
		panic("server: nil architecture")
	}

	architecturesMu.Lock()
	defer architecturesMu.Unlock()
	registeredArchitectures[name] = implementation
	registeredArchitectureDB = architecture.NewRegistry(registeredArchitectures)
}

func GetArchitectureList(c *gin.Context) {
	displayLanguage := c.Query("display_language")
	response := make([]architectureMetadataResponse, 0)
	for _, name := range registeredArchitectureNames() {
		arch, ok := getArchitecture(name)
		if !ok {
			writeArchitectureError(c, newUnknownArchError(name), name)
			return
		}
		metadata, err := arch.GetMetadata(displayLanguage)
		if err != nil {
			writeArchitectureError(c, err, name)
			return
		}
		response = append(response, newArchitectureMetadataResponse(name, metadata))
	}
	c.JSON(http.StatusOK, response)
}

func GetArchitecture(c *gin.Context) {
	name := c.Param("arch_id")
	arch, ok := getArchitecture(name)
	if !ok {
		writeArchitectureError(c, newUnknownArchError(name), name)
		return
	}
	metadata, err := arch.GetMetadata(c.Query("display_language"))
	if err != nil {
		writeArchitectureError(c, err, name)
		return
	}
	c.JSON(http.StatusOK, newArchitectureMetadataResponse(name, metadata))
}

func getArchitecture(name string) (architecture.Architecture, bool) {
	architecturesMu.RLock()
	defer architecturesMu.RUnlock()
	return registeredArchitectureDB.Get(name)
}

func registeredArchitectureNames() []string {
	architecturesMu.RLock()
	defer architecturesMu.RUnlock()
	return registeredArchitectureDB.Names()
}

func newUnknownArchError(arch string) error {
	return api.NewUnknownArchError(arch, "supported architectures: "+strings.Join(registeredArchitectureNames(), ", "))
}

func newArchitectureMetadataResponse(id string, metadata api.ArchitectureMetadata) architectureMetadataResponse {
	return architectureMetadataResponse{
		ID:                id,
		Name:              metadata.Name,
		PronunciationMode: metadata.PronunciationMode,
		PhonemeMode:       metadata.PhonemeMode,
		Parameters:        metadata.Parameters,
		AudioDependencies: metadata.AudioDependencies,
	}
}

func writeArchitectureError(c *gin.Context, err error, arch string) {
	writeProblem(c, err, problemContext{Arch: arch})
}
