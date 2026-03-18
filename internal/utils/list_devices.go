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
	"diffscope-synthesis-platform/native"
	"encoding/json"
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
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

func toJSONDeviceInfo(device native.DeviceInfo) jsonDeviceInfo {
	return jsonDeviceInfo{
		Type:        device.Type().String(),
		Index:       device.Index(),
		Description: device.Description(),
		ID:          device.Id(),
		Memory:      device.Memory(),
	}
}

func formatMemorySize(memory uint64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case memory >= GB:
		return fmt.Sprintf("%.2f GiB", float64(memory)/float64(GB))
	case memory >= MB:
		return fmt.Sprintf("%.2f MiB", float64(memory)/float64(MB))
	case memory >= KB:
		return fmt.Sprintf("%.2f KiB", float64(memory)/float64(KB))
	default:
		return fmt.Sprintf("%d B", memory)
	}
}

func ListDevices(shouldPrintAsJson bool) {
	executionProviders := native.ExecutionProviderInfoGetExecutionProviders()
	defaultDevice := native.ExecutionProviderInfoGetDefaultDevice()
	if shouldPrintAsJson {
		response := jsonListDevicesResponse{
			ExecutionProviders: make([]jsonExecutionProviderInfo, 0, executionProviders.Size()),
			DefaultDevice:      toJSONDeviceInfo(defaultDevice),
		}

		for i := int64(0); i < executionProviders.Size(); i++ {
			provider := executionProviders.Get(int(i))
			providerDevices := provider.Devices()
			jsonProvider := jsonExecutionProviderInfo{
				Type:    provider.Type().String(),
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
		twStyle := table.StyleRounded
		twStyle.Options.SeparateRows = true
		twStyle.Format.Header = text.FormatDefault
		for i := int64(0); i < executionProviders.Size(); i++ {
			provider := executionProviders.Get(int(i))
			providerType := provider.Type()
			tw := table.NewWriter()
			tw.SetStyle(twStyle)
			providerName := providerType.String()

			if providerType == native.ExecutionProviderType_CPU || providerType == native.ExecutionProviderType_CoreML {
				tw.AppendRow(
					table.Row{providerName},
				)
				fmt.Println(tw.Render())
				continue
			} else {
				tw.AppendHeader(
					table.Row{providerName, providerName, providerName, providerName},
					table.RowConfig{AutoMerge: true},
				)
			}

			tw.AppendHeader(table.Row{"Index", "Description", "ID", "Memory"})

			devices := provider.Devices()
			for j := int64(0); j < devices.Size(); j++ {
				device := devices.Get(int(j))
				tw.AppendRow(table.Row{
					device.Index(),
					device.Description(),
					device.Id(),
					formatMemorySize(device.Memory()),
				})
			}

			fmt.Println(tw.Render())
		}
		fmt.Println()
		fmt.Printf("Default device:\n")
		tw := table.NewWriter()
		tw.SetStyle(twStyle)
		providerType := defaultDevice.Type()
		providerName := providerType.String()
		if providerType == native.ExecutionProviderType_CPU || providerType == native.ExecutionProviderType_CoreML {
			tw.AppendRow(
				table.Row{providerName},
			)
			fmt.Println(tw.Render())
		} else {
			tw.AppendHeader(
				table.Row{providerName, providerName, providerName, providerName},
				table.RowConfig{AutoMerge: true},
			)
			tw.AppendHeader(table.Row{"Index", "Description", "ID", "Memory"})
			tw.AppendRow(table.Row{
				defaultDevice.Index(),
				defaultDevice.Description(),
				defaultDevice.Id(),
				formatMemorySize(defaultDevice.Memory()),
			})
			fmt.Println(tw.Render())
		}
	}
}
