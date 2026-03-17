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

#include "ExecutionProviderInfo.h"

#include <algorithm>
#include <iomanip>
#include <sstream>

#ifdef WIN32
#  include <dxgi1_6.h>
#  include <wrl/client.h>
#endif

#ifdef WIN32

using Microsoft::WRL::ComPtr;

static std::string wstrToString(const wchar_t *wstr) {
	if (wstr == nullptr || *wstr == L'\0') {
		return {};
	}

	const int requiredSize = WideCharToMultiByte(
		CP_UTF8,
		0,
		wstr,
		-1,
		nullptr,
		0,
		nullptr,
		nullptr
	);
	if (requiredSize <= 0) {
		return {};
	}

	std::string result(static_cast<size_t>(requiredSize - 1), '\0');
	const int convertedSize = WideCharToMultiByte(
		CP_UTF8,
		0,
		wstr,
		-1,
		result.data(),
		requiredSize,
		nullptr,
		nullptr
	);
	if (convertedSize <= 0) {
		return {};
	}

	return result;
}

static std::string getDmlDeviceId(const DXGI_ADAPTER_DESC1 &desc) {
	std::stringstream ss;
	ss << std::setfill('0') << std::setw(8) << std::hex << desc.VendorId;
	ss << "-";
	ss << std::setfill('0') << std::setw(8) << std::hex << desc.DeviceId;
	return ss.str();
}

#endif

std::vector<ExecutionProviderInfo> ExecutionProviderInfo::GetExecutionProviders() {
	std::vector<ExecutionProviderInfo> list;

	static ExecutionProviderInfo cpuExecutionProvider {
		ExecutionProviderType::CPU,
		{
			DeviceInfo {
				ExecutionProviderType::CPU,
				0,
				{},
				{},
				0,
			}
		}
	};
	list.push_back(cpuExecutionProvider);

	// TODO cuda

#ifdef WIN32
	std::vector<DeviceInfo> dmlDevices;
	ComPtr<IDXGIFactory6> dxgiFactory;
	if (!FAILED(CreateDXGIFactory1(IID_PPV_ARGS(&dxgiFactory)))) {
		ComPtr<IDXGIAdapter1> adapter;
		for (int adapterIndex = 0; dxgiFactory->EnumAdapters1(adapterIndex, &adapter) != DXGI_ERROR_NOT_FOUND; ++adapterIndex) {
			DXGI_ADAPTER_DESC1 desc;
			if (FAILED(adapter->GetDesc1(&desc))) {
				continue;
			}

			if (desc.Flags & DXGI_ADAPTER_FLAG_SOFTWARE) {
				// Skip software adapters
				continue;
			}
			dmlDevices.push_back({
				ExecutionProviderType::DirectML,
				adapterIndex,
				wstrToString(desc.Description),
				getDmlDeviceId(desc),
				desc.DedicatedVideoMemory
			});
		}
	}
	// Sort gpuList by DedicatedVideoMemory in descending order
	std::ranges::sort(dmlDevices, [](const auto &a, const auto &b) { return a.Memory() > b.Memory(); });
	list.push_back({
		ExecutionProviderType::DirectML,
		std::move(dmlDevices)
	});
#endif

#ifdef __APPLE__
	static ExecutionProviderInfo coreMLExecutionProvider {
		ExecutionProviderType::CoreML,
		{
			DeviceInfo {
				ExecutionProviderType::CoreML,
				0,
				{},
				{},
				0,
			}
		}
	};
	list.push_back(coreMLExecutionProvider);
#endif

	return list;
}

DeviceInfo ExecutionProviderInfo::GetDefaultDevice() {
	static DeviceInfo cpuDeviceInfo {
		ExecutionProviderType::CPU,
		0,
		{},
		{},
		0,
	};
#ifdef WIN32
	ComPtr<IDXGIFactory6> dxgiFactory;
	if (FAILED(CreateDXGIFactory(IID_PPV_ARGS(&dxgiFactory)))) {
		return cpuDeviceInfo;
	}

	auto preferredDeviceInfo = cpuDeviceInfo;
	auto alternativeDeviceInfo = cpuDeviceInfo;

	// Enumerate adapters
	ComPtr<IDXGIAdapter1> adapter;
	for (int adapterIndex = 0; dxgiFactory->EnumAdapters1(adapterIndex, &adapter) != DXGI_ERROR_NOT_FOUND; ++adapterIndex) {
		DXGI_ADAPTER_DESC1 desc;
		if (FAILED(adapter->GetDesc1(&desc))) {
			continue;
		}

		if (desc.Flags & DXGI_ADAPTER_FLAG_SOFTWARE) {
			// Skip software adapters
			continue;
		}

		enum VendorIdList : unsigned int {
			VID_AMD = 0x1002,
			VID_NVIDIA = 0x10DE,
			VID_SAPPHIRE = 0x174B,
		};

		DeviceInfo deviceInfo {
			ExecutionProviderType::DirectML,
			adapterIndex,
			wstrToString(desc.Description),
			getDmlDeviceId(desc),
			desc.DedicatedVideoMemory
		};

		bool mayBeDedicatedGPU =
			desc.VendorId == VID_NVIDIA ||
			desc.VendorId == VID_AMD ||
			desc.VendorId == VID_SAPPHIRE ||
			[desc] {
				auto s = wstrToString(desc.Description);
				std::ranges::transform(s, s.begin(), [](unsigned char c) { return std::toupper(c); });
				return s == "NVIDIA";
			}();
		if (mayBeDedicatedGPU) {
			if (preferredDeviceInfo.Type() == ExecutionProviderType::CPU || desc.DedicatedVideoMemory > preferredDeviceInfo.Memory()) {
				preferredDeviceInfo = deviceInfo;
			}
		} else {
			if (alternativeDeviceInfo.Type() == ExecutionProviderType::CPU || desc.DedicatedVideoMemory > alternativeDeviceInfo.Memory()) {
				alternativeDeviceInfo = deviceInfo;
			}
		}
	}
	return preferredDeviceInfo.Type() != ExecutionProviderType::CPU ? preferredDeviceInfo : alternativeDeviceInfo;

#endif

#ifdef __APPLE__
	return {
		ExecutionProviderType::CoreML,
		0,
		{},
		{},
		0,
	}
#endif

	// TODO cuda

	return cpuDeviceInfo;
}
