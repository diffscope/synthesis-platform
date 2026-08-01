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

package diffsinger

import (
	"encoding/json"
	"fmt"

	"diffscope-synthesis-platform/internal/api"
	"diffscope-synthesis-platform/internal/executionprovider"
	"diffscope-synthesis-platform/internal/languageconversion"
	"diffscope-synthesis-platform/internal/phonemeconversion"
	"diffscope-synthesis-platform/internal/server"
	"diffscope-synthesis-platform/internal/synthrt"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

type Architecture struct{}

type SingerExtra struct {
	Speaker string `json:"speaker"`
}

type ArchExtra struct {
	Steps int64   `json:"steps"`
	Depth float32 `json:"depth"`
}

type singerExtraRequest struct {
	Speaker *string `json:"speaker" validate:"required"`
}

type archExtraRequest struct {
	Steps *int64   `json:"steps"`
	Depth *float32 `json:"depth"`
}

const (
	defaultArchExtraSteps int64   = 20
	defaultArchExtraDepth float32 = 1
)

var extraValidator = validator.New()

func (Architecture) GetMetadata(displayLanguage string) (api.ArchitectureMetadata, error) {
	return api.ArchitectureMetadata{
		Name:              "DiffSinger",
		PronunciationMode: api.ArchitectureSynthesisModeFull,
		PhonemeMode:       api.ArchitectureSynthesisModeFull,
		Parameters: map[string]api.ArchitectureParameterMetadata{
			parameterIDExpressiveness: {
				Type: api.ArchitectureParameterTypeDirect,
			},
			parameterIDPitch: {
				Type:      api.ArchitectureParameterTypeIndirect,
				DependsOn: []string{parameterIDExpressiveness},
			},
			parameterIDBreathiness: {
				Type:      api.ArchitectureParameterTypeIndirect,
				DependsOn: []string{parameterIDPitch},
			},
			parameterIDTension: {
				Type:      api.ArchitectureParameterTypeIndirect,
				DependsOn: []string{parameterIDPitch},
			},
			parameterIDVoicing: {
				Type:      api.ArchitectureParameterTypeIndirect,
				DependsOn: []string{parameterIDPitch},
			},
			parameterIDEnergy: {
				Type:      api.ArchitectureParameterTypeIndirect,
				DependsOn: []string{parameterIDPitch},
			},
			parameterIDMouthOpening: {
				Type:      api.ArchitectureParameterTypeIndirect,
				DependsOn: []string{parameterIDPitch},
			},
			parameterIDGender: {
				Type: api.ArchitectureParameterTypeDirect,
			},
			parameterIDVelocity: {
				Type: api.ArchitectureParameterTypeDirect,
			},
			parameterIDToneShift: {
				Type: api.ArchitectureParameterTypeDirect,
			},
		},
		AudioDependencies: []string{
			parameterIDPitch,
			parameterIDBreathiness,
			parameterIDTension,
			parameterIDVoicing,
			parameterIDEnergy,
			parameterIDMouthOpening,
			parameterIDGender,
			parameterIDVelocity,
			parameterIDToneShift,
		},
	}, nil
}

func (r archExtraRequest) ToArchExtra() ArchExtra {
	extra := ArchExtra{
		Steps: defaultArchExtraSteps,
		Depth: defaultArchExtraDepth,
	}
	if r.Steps != nil {
		extra.Steps = *r.Steps
	}
	if r.Depth != nil {
		extra.Depth = *r.Depth
	}
	return extra
}

func (r singerExtraRequest) ToSingerExtra() SingerExtra {
	return SingerExtra{Speaker: *r.Speaker}
}

func parseArchExtra(data json.RawMessage) (ArchExtra, error) {
	var request archExtraRequest
	if err := json.Unmarshal(data, &request); err != nil {
		detail := fmt.Sprintf("parse arch extra: %v", err)
		return ArchExtra{}, api.NewSingerConfigInvalidError(detail, api.ValidationIssue{
			Pointer: "#/context/arch_extra",
			Type:    "invalid_json",
			Detail:  err.Error(),
		})
	}
	if err := extraValidator.Struct(request); err != nil {
		detail := fmt.Sprintf("validate arch extra: %v", err)
		return ArchExtra{}, api.NewSingerConfigInvalidError(detail, api.ValidationIssue{
			Pointer: "#/context/arch_extra",
			Type:    "invalid_value",
			Detail:  err.Error(),
		})
	}
	return request.ToArchExtra(), nil
}

func parseSingerExtra(data json.RawMessage) (SingerExtra, error) {
	var request singerExtraRequest
	if err := json.Unmarshal(data, &request); err != nil {
		detail := fmt.Sprintf("parse singer extra: %v", err)
		return SingerExtra{}, api.NewSingerConfigInvalidError(detail, api.ValidationIssue{
			Pointer: "#/context",
			Type:    "invalid_singer_extra",
			Detail:  err.Error(),
		})
	}
	if err := extraValidator.Struct(request); err != nil {
		detail := fmt.Sprintf("validate singer extra: %v", err)
		return SingerExtra{}, api.NewSingerConfigInvalidError(detail, api.ValidationIssue{
			Pointer: "#/context",
			Type:    "required",
			Detail:  "singer extra must identify a speaker",
		})
	}
	return request.ToSingerExtra(), nil
}

func validateSingerExtraSpeaker(metadata SingerMetadata, speaker string) error {
	if len(metadata.Speakers) == 0 {
		if speaker != "" {
			return api.NewError(api.ProblemTypeInternalError, "singer does not define speakers")
		}
		return nil
	}
	if _, ok := metadata.Speakers[speaker]; !ok {
		detail := fmt.Sprintf("speaker %q does not exist", speaker)
		return api.NewSingerConfigInvalidError(detail, api.ValidationIssue{
			Pointer: "#/context",
			Type:    "unknown_speaker",
			Detail:  detail,
		})
	}
	return nil
}

func init() {
	server.RegisterArchitecture("diffsinger", Architecture{})
	server.RegisterStartRoutine(func() error {
		device, err := executionprovider.ConfiguredDevice()
		if err != nil {
			return err
		}
		packageDir := viper.GetString("package_dir")
		if err := synthrt.Initialize(packageDir, device); err != nil {
			return err
		}
		if err := languageconversion.Initialize(device); err != nil {
			return err
		}
		if err := RefreshSingerRegistry(packageDir); err != nil {
			return err
		}
		phonemeconversion.SetLuaRunnerCount(getPhonemeCustomWorkerCount())
		configurePhonemeResourceManagers()
		configureDurationResourceManager()
		configureParameterResourceManager()
		configureAudioResourceManagers()
		return nil
	})
}
