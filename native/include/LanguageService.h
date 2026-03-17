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
#include <utility>

enum class ExecutionProviderType;

struct LanguageServiceInitializationError {
	std::string Error() const {
		return m_message;
	}
private:
	explicit LanguageServiceInitializationError(std::string message) : m_message(std::move(message)) {}
	std::string m_message;
};

class LanguageService {
public:
	static LanguageServiceInitializationError *initialize(ExecutionProviderType ep, int deviceIndex);
};

#endif //DSSP_LANGUAGESERVICE_H