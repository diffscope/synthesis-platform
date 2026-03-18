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

#include "LanguageService.h"

#include <filesystem>

#include <stdcorelib/str.h>
#include <stdcorelib/system.h>

#include <LangCore/Core/Manager.h>

#include <ExecutionProviderInfo.h>

std::filesystem::path getPluginRootDirectory() {
#if defined(__APPLE__)
    return stdc::system::application_directory().parent_path() / _TSTR("PlugIns/LangPlugins");
#else
    return stdc::system::application_directory().parent_path() / _TSTR("lib/plugins/LangPlugins");
#endif
}

std::filesystem::path getPackagesRootDirectory() {
#if defined(__APPLE__)
    return stdc::system::application_directory().parent_path() / _TSTR("Resources/G2pPackages");
#else
    return stdc::system::application_directory().parent_path() / _TSTR("share/G2pPackages");
#endif
}

const LanguageServiceInitializationError *LanguageService::Initialize(ExecutionProviderType ep, int deviceIndex) {
    const auto langMgr = LangCore::Manager::instance();

    const auto defaultPluginDir = getPluginRootDirectory() ;
    langMgr->addPluginPath("org.openvpi.DriverFactory", defaultPluginDir / _TSTR("Drivers"));
    langMgr->addPluginPath("org.openvpi.TaskFactory", defaultPluginDir / _TSTR("G2ps"));
    langMgr->addPluginPath("org.openvpi.TaskFactory", defaultPluginDir / _TSTR("Taggers"));
    langMgr->addPluginPath("org.openvpi.TaskFactory", defaultPluginDir / _TSTR("Splitters"));

    const std::filesystem::path packagesRootDir = getPackagesRootDirectory();
    langMgr->addPackagePath(packagesRootDir);

    // if (const auto onnxDriverInitialized = initializeOnnxDriver(langMgr, "cpu", 0, true); !onnxDriverInitialized)
    //     std::cerr << "Failed to initializeOnnxDriver" << std::endl;

    std::string msg;
    langMgr->initialize(msg);

    static LanguageServiceInitializationError error;
    if (langMgr->initialized()) {
        return nullptr;
    } else {
        error.m_message = msg;
        return &error;
    }
}