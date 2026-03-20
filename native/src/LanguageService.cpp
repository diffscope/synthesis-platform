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

#include <algorithm>
#include <filesystem>

#include <stdcorelib/str.h>
#include <stdcorelib/system.h>

#include <LangCore/Core/Manager.h>
#include <LangCore/Module/Module.h>
#include <LangCore/Task/TaskFactoryPlugin.h>

#include <LangPlugins/Api/Drivers/Onnx/1/OnnxDriverApiL1.h>

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

LangPlugins::Api::Onnx::L1::ExecutionProvider parseExecutionProvider(ExecutionProviderType ep) {
    if (ep == ExecutionProviderType::CPU) {
        return LangPlugins::Api::Onnx::L1::ExecutionProvider::CPUExecutionProvider;
    }
    if (ep == ExecutionProviderType::CUDA) {
        return LangPlugins::Api::Onnx::L1::ExecutionProvider::CUDAExecutionProvider;
    }
    if (ep == ExecutionProviderType::DirectML) {
        return LangPlugins::Api::Onnx::L1::ExecutionProvider::DMLExecutionProvider;
    }
    if (ep == ExecutionProviderType::CoreML) {
        return LangPlugins::Api::Onnx::L1::ExecutionProvider::CoreMLExecutionProvider;
    }
    return LangPlugins::Api::Onnx::L1::ExecutionProvider::CPUExecutionProvider;
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

    const auto onnxDriverPlugin = langMgr->plugin<LangCore::DriverFactoryPlugin>("onnx");
    if (!onnxDriverPlugin) {
        std::cerr << "Failed to load ONNX inference driver" << std::endl;
    } else {
        const auto onnxDriver = onnxDriverPlugin->create();
        const auto onnxArgs = LangCore::NO<LangPlugins::Api::Onnx::L1::DriverInitArgs>::create();

        const auto ep_ = parseExecutionProvider(ep);
        onnxArgs->ep = ep_;
        const auto ortParentPath = onnxDriverPlugin->path().parent_path() / _TSTR("runtimes") / _TSTR("onnx");
        onnxArgs->runtimePath = ep_ == LangPlugins::Api::Onnx::L1::CUDAExecutionProvider ? ortParentPath / _TSTR("cuda") : ortParentPath / _TSTR("default");

        onnxArgs->loadFromProcess = false;
        onnxArgs->deviceIndex = deviceIndex;

        if (const auto exp = onnxDriver->initialize(onnxArgs); !exp) {
            std::cerr << "Failed to initialize ONNX driver: " << exp.error().message() << std::endl;
        }

        auto &driverCategory = *langMgr->category("driver");
        driverCategory.addObject("g2pOnnxDriver", onnxDriver);
    }

    std::string msg;
    langMgr->initialize(msg);

    if (langMgr->initialized()) {
        return nullptr;
    } else {
        auto error = new LanguageServiceInitializationError;
        error->m_message = msg;
        return error;
    }
}

std::vector<std::string> LanguageService::Split_ReturnValueNeedsDeferDelete(const std::vector<std::string> &input) {
    return LangCore::Manager::instance()->split(input);
}

void LanguageService::TagInPlace(const std::vector<LanguageServiceTaggedNote *> &input, const std::vector<std::string> &preferredLanguages, const std::vector<std::string> &graphemeTypePriority) {
    auto langMgr = LangCore::Manager::instance();
    auto originalOrder = langMgr->defaultTaggerOrder();
    if (!graphemeTypePriority.empty())
        langMgr->setDefaultOrder(graphemeTypePriority);
    std::vector<std::string> input_;
    input_.reserve(input.size());
    std::ranges::transform(input, std::back_inserter(input_), [](LanguageServiceTaggedNote *x) {
        return x->Lyric();
    });
    auto output = langMgr->tag(input_, false, false, preferredLanguages);
    assert(output.size() == input.size());
    for (size_t i = 0; i < output.size(); i++) {
        input[i]->m_language = std::move(output[i].language);
        input[i]->m_lyric = std::move(output[i].lyric);
        input[i]->m_graphemeType = std::move(output[i].tag);
        input[i]->m_nonTextOmittable = output[i].discard;
    }
    langMgr->setDefaultOrder(originalOrder);
}

void LanguageService::ConvertInPlace(const std::vector<LanguageServiceConvertedNote *> &input) {
    auto langMgr = LangCore::Manager::instance();
    std::vector<std::unique_ptr<LangCore::G2pInput>> input_;
    input_.reserve(input.size());
    std::ranges::transform(input, std::back_inserter(input_), [](LanguageServiceConvertedNote *x) {
        return std::make_unique<LangCore::G2pInput>(x->Lyric(), x->PronunciationType());
    });
    std::vector<LangCore::G2pInput *> ptrInput;
    ptrInput.reserve(input_.size());
    std::ranges::transform(input_, std::back_inserter(ptrInput), [](auto &x) {
        return x.get();
    });
    auto output = langMgr->convert(ptrInput);
    assert(output.size() == input.size());
    for (size_t i = 0; i < output.size(); i++) {
        input[i]->m_lyric = std::move(output[i].lyric);
        input[i]->m_pronunciationType = std::move(output[i].g2pId);
        input[i]->m_pronunciation = std::move(output[i].pronunciation);
        input[i]->m_candidatePronunciations = std::move(output[i].candidates);
        input[i]->m_error = output[i].error;
    }
}
