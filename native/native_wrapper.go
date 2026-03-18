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

package native

import (
	"fmt"
	"strings"
)

func (e ExecutionProviderType) String() string {
	switch e {
	case ExecutionProviderType_CPU:
		return "cpu"
	case ExecutionProviderType_CUDA:
		return "cuda"
	case ExecutionProviderType_DirectML:
		return "directml"
	case ExecutionProviderType_CoreML:
		return "coreml"
	}
	panic(fmt.Sprintf("Unreachable invalid ExecutionProviderType: %d", e))
}

func ExecutionProviderTypeFromString(s string) (ExecutionProviderType, error) {
	switch strings.ToLower(s) {
	case "cpu":
		return ExecutionProviderType_CPU, nil
	case "cuda":
		return ExecutionProviderType_CUDA, nil
	case "directml":
		return ExecutionProviderType_DirectML, nil
	case "coreml":
		return ExecutionProviderType_CoreML, nil
	}
	return 0, fmt.Errorf("Invalid execution provider type: %s", s)
}
