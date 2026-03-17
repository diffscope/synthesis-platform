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

package utils

import (
	"diffscope-synthesis-platform/native/bindings"
	"encoding/json"
	"os"
)

type jsonDeviceInfo struct {
	Type        string `json:"type"`
	Index       int    `json:"index"`
	Description string `json:"description"`
	ID          string `json:"id"`
	Memory      uint64 `json:"memory"`
}

type jsonExecutionProviderInfo struct {
	Type    string           `json:"type"`
	Devices []jsonDeviceInfo `json:"devices"`
}

type jsonListDevicesResponse struct {
	ExecutionProviders []jsonExecutionProviderInfo `json:"execution_providers"`
	DefaultDevice      jsonDeviceInfo              `json:"default_device"`
}

func executionProviderTypeToString(providerType bindings.ExecutionProviderType) string {
	switch providerType {
	case bindings.ExecutionProviderType_CPU:
		return "cpu"
	case bindings.ExecutionProviderType_CUDA:
		return "cuda"
	case bindings.ExecutionProviderType_DirectML:
		return "directml"
	case bindings.ExecutionProviderType_CoreML:
		return "coreml"
	default:
		panic("Unknown execution provider")
	}
}

func toJSONDeviceInfo(device bindings.DeviceInfo) jsonDeviceInfo {
	return jsonDeviceInfo{
		Type:        executionProviderTypeToString(device.Type()),
		Index:       device.Index(),
		Description: device.Description(),
		ID:          device.Id(),
		Memory:      device.Memory(),
	}
}

func ListDevices(shouldPrintAsJson bool) {
	executionProviders := bindings.ExecutionProviderInfoGetExecutionProviders()
	defaultDevice := bindings.ExecutionProviderInfoGetDefaultDevice()
	if shouldPrintAsJson {
		response := jsonListDevicesResponse{
			ExecutionProviders: make([]jsonExecutionProviderInfo, 0, executionProviders.Size()),
			DefaultDevice:      toJSONDeviceInfo(defaultDevice),
		}

		for i := int64(0); i < executionProviders.Size(); i++ {
			provider := executionProviders.Get(int(i))
			providerDevices := provider.Devices()
			jsonProvider := jsonExecutionProviderInfo{
				Type:    executionProviderTypeToString(provider.Type()),
				Devices: make([]jsonDeviceInfo, 0, providerDevices.Size()),
			}

			for j := int64(0); j < providerDevices.Size(); j++ {
				jsonProvider.Devices = append(jsonProvider.Devices, toJSONDeviceInfo(providerDevices.Get(int(j))))
			}

			response.ExecutionProviders = append(response.ExecutionProviders, jsonProvider)
		}

		encoder := json.NewEncoder(os.Stdout)
		if err := encoder.Encode(response); err != nil {
			panic(err)
		}
	} else {

	}
}
