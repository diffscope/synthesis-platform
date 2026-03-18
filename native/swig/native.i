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

%module native

%{
#include "ExecutionProviderInfo.h"
#include "LanguageService.h"
%}

%include "stdint.i"
%include "std_string.i"
%include "std_vector.i"

%include "ExecutionProviderInfo.h"
%include "LanguageService.h"

%template(StringVector) std::vector<std::string>;
%template(ExecutionProviderInfoVector) std::vector<ExecutionProviderInfo>;
%template(DeviceInfoVector) std::vector<DeviceInfo>;
%template(LanguageServiceTaggedNoteVector) std::vector<LanguageServiceTaggedNote *>;
%template(LanguageServiceConvertedNoteVector) std::vector<LanguageServiceConvertedNote *>;
