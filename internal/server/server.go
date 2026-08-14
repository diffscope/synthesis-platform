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
	"fmt"
	"sync"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

type StartRoutine func() error

var (
	startRoutinesMu sync.Mutex
	startRoutines   []StartRoutine
)

func RegisterStartRoutine(routine StartRoutine) {
	if routine == nil {
		panic("server: nil start routine")
	}

	startRoutinesMu.Lock()
	defer startRoutinesMu.Unlock()
	startRoutines = append(startRoutines, routine)
}

func runStartRoutines() error {
	startRoutinesMu.Lock()
	routines := append([]StartRoutine(nil), startRoutines...)
	startRoutinesMu.Unlock()

	for _, routine := range routines {
		if err := routine(); err != nil {
			return err
		}
	}
	return nil
}

func StartRouter() error {
	router := gin.Default()
	router.Use(gzip.Gzip(gzip.DefaultCompression))

	router.GET("/v1/info", GetApplicationInfo)
	router.GET("/v1/arch", GetArchitectureList)
	router.GET("/v1/arch/:arch_id", GetArchitecture)
	router.GET("/v1/singer", GetSingerList)
	router.GET("/v1/arch/:arch_id/singer", GetArchSingerList)
	router.GET("/v1/arch/:arch_id/singer/:singer_id", GetArchSinger)
	router.GET("/v1/arch/:arch_id/singer/:singer_id/avatar", GetArchSingerAvatar)
	router.GET("/v1/arch/:arch_id/singer/:singer_id/background", GetArchSingerBackground)
	router.GET("/v1/arch/:arch_id/singer/:singer_id/demo_audio", GetArchSingerDemoAudioList)
	router.GET("/v1/extractor", GetExtractorList)
	router.GET("/v1/extractor/note/:extractor_id", GetNoteExtractor)
	router.GET("/v1/extractor/tempo/:extractor_id", GetTempoExtractor)
	router.GET("/v1/extractor/pitch/:extractor_id", GetPitchExtractor)
	router.POST("/v1/env_tag", PostEnvTag)
	router.POST("/v1/synth/pronunciation", PostPronunciation)
	router.POST("/v1/synth/phoneme", PostPhoneme)
	router.POST("/v1/synth/duration", PostDuration)
	router.POST("/v1/synth/parameter", PostParameter)
	router.POST("/v1/synth/audio", PostAudio)
	router.POST("/v1/extract/note", PostExtractNote)
	router.POST("/v1/extract/tempo", PostExtractTempo)
	router.POST("/v1/extract/pitch", PostExtractPitch)

	host := viper.GetString("host")
	port := viper.GetInt("port")

	return router.Run(fmt.Sprintf("%s:%d", host, port))
}

func StartServer() error {
	if err := runStartRoutines(); err != nil {
		return err
	}
	return StartRouter()
}
