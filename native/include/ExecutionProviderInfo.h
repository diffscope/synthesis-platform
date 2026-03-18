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

#ifndef DSSP_EXECUTIONPROVIDERINFO_H
#define DSSP_EXECUTIONPROVIDERINFO_H

#include <cstdint>
#include <string>
#include <vector>

class DeviceInfo;

enum class ExecutionProviderType {
	CPU,
	CUDA,
	DirectML,
	CoreML,
};

class ExecutionProviderInfo {
public:
	inline ExecutionProviderInfo();

	ExecutionProviderType Type() const {
		return m_type;
	}

	const std::vector<DeviceInfo> &Devices() const {
		return m_devices;
	}

	static const std::vector<ExecutionProviderInfo> &GetExecutionProviders();
	static const DeviceInfo &GetDefaultDevice();

private:
	inline ExecutionProviderInfo(ExecutionProviderType type, std::vector<DeviceInfo> devices);

	ExecutionProviderType m_type;
	std::vector<DeviceInfo> m_devices;
};

class DeviceInfo {
public:
	DeviceInfo() : m_type(ExecutionProviderType::CPU), m_index(0), m_memory(0) {
	}
	ExecutionProviderType Type() const {
		return m_type;
	}

	int Index() const {
		return m_index;
	}

	std::string Description() const {
		return m_description;
	}

	std::string Id() const {
		return m_id;
	}

	uint64_t Memory() const {
		return m_memory;
	}

private:
	friend class ExecutionProviderInfo;
	DeviceInfo(ExecutionProviderType type, int index, std::string description, std::string id, uint64_t memory) : m_type(type), m_index(index), m_description(std::move(description)), m_id(std::move(id)), m_memory(memory) {
	}
	ExecutionProviderType m_type;
	int m_index;
	std::string m_description;
	std::string m_id;
	uint64_t m_memory;
};

ExecutionProviderInfo::ExecutionProviderInfo() : m_type(ExecutionProviderType::CPU), m_devices({DeviceInfo{}}) {
}

ExecutionProviderInfo::ExecutionProviderInfo(ExecutionProviderType type, std::vector<DeviceInfo> devices) : m_type(type), m_devices(std::move(devices)) {
}

#endif // DSSP_EXECUTIONPROVIDERINFO_H
