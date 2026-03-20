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

#ifndef DSSP_LANGUAGESERVICE_H
#define DSSP_LANGUAGESERVICE_H

#include <string>
#include <vector>

enum class ExecutionProviderType;

struct LanguageServiceInitializationError {
	std::string Error() const {
		return m_message;
	}
private:
	friend class LanguageService;
	std::string m_message;
};

struct LanguageServiceTaggedNote {
	std::string Language() const {
		return m_language;
	}
	std::string Lyric() const {
		return m_lyric;
	}
	void SetLyric(std::string lyric) {
		m_lyric = std::move(lyric);
	}
	std::string GraphemeType() const {
		return m_graphemeType;
	}
	bool IsNonTextOmittable() const {
		return m_nonTextOmittable;
	}
private:
	friend class LanguageService;
	std::string m_language;
	std::string m_lyric;
	std::string m_graphemeType;
	bool m_nonTextOmittable;
};

struct LanguageServiceConvertedNote {
	std::string Lyric() const {
		return m_lyric;
	}
	void SetLyric(std::string lyric) {
		m_lyric = std::move(lyric);
	}
	std::string PronunciationType() const {
		return m_pronunciationType;
	}
	void SetPronunciationType(std::string pronunciationType) {
		m_pronunciationType = std::move(pronunciationType);
	}
	std::string Pronunciation() const {
		return m_pronunciation;
	}
	const std::vector<std::string> &CandidatePronunciations() const {
		return m_candidatePronunciations;
	}
	bool IsError() const {
		return m_error;
	}
private:
	friend class LanguageService;
	std::string m_lyric;
	std::string m_pronunciationType;
	std::string m_pronunciation;
	std::vector<std::string> m_candidatePronunciations;
	bool m_error;
};

class LanguageService {
public:
	static const LanguageServiceInitializationError *Initialize(ExecutionProviderType ep, int deviceIndex);
	static std::vector<std::string> Split_ReturnValueNeedsDeferDelete(const std::vector<std::string> &input);
	static void TagInPlace(const std::vector<LanguageServiceTaggedNote *> &input, const std::vector<std::string> &preferredLanguages, const std::vector<std::string> &graphemeTypePriority);
	static void ConvertInPlace(const std::vector<LanguageServiceConvertedNote *> &input);
};

#endif //DSSP_LANGUAGESERVICE_H