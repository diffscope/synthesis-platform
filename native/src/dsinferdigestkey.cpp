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

#include "native.h"

#include <array>
#include <string>

#include <synthrt/SVS/InferenceContrib.h>

namespace dssp {

	namespace {
		thread_local std::string g_inferenceDigestKey;

		void appendInferenceFullID(std::string &result, const srt::InferenceSpec *inference) {
			if (!result.empty()) {
				result += ',';
			}

			const auto &package = inference->parent();
			result += package.id();
			result += '@';
			result += package.version().toString();
			result += ':';
			result += inference->id();
		}

	} // namespace

} // namespace dssp

const char *DSSP_GetDiffSingerInferenceDigestKey(DSSP_SRTSinger singer) {
	const std::array inferences{
		static_cast<srt::InferenceSpec *>(DSSP_GetDiffSingerDurationInference(singer)),
		static_cast<srt::InferenceSpec *>(DSSP_GetDiffSingerPitchInference(singer)),
		static_cast<srt::InferenceSpec *>(DSSP_GetDiffSingerVarianceInference(singer)),
		static_cast<srt::InferenceSpec *>(DSSP_GetDiffSingerAcousticInference(singer)),
		static_cast<srt::InferenceSpec *>(DSSP_GetDiffSingerVocoderInference(singer)),
	};

	dssp::g_inferenceDigestKey.clear();
	for (const auto *inference : inferences) {
		dssp::appendInferenceFullID(dssp::g_inferenceDigestKey, inference);
	}
	return dssp::g_inferenceDigestKey.c_str();
}

