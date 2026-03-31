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
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type PackageVersion struct {
	Major int
	Minor int
	Patch int
	Tweak int
}

func (v PackageVersion) String() string {
	return fmt.Sprintf("%d.%d.%d.%d", v.Major, v.Minor, v.Patch, v.Tweak)
}

func ParsePackageVersion(versionStr string) (*PackageVersion, error) {
	pattern := `^\d{1,4}(?:\.\d{1,4}){0,3}$`
	matched, err := regexp.MatchString(pattern, versionStr)
	if err != nil {
		return nil, fmt.Errorf("validate version format: %w", err)
	}
	if !matched {
		return nil, fmt.Errorf("invalid version format: %s", versionStr)
	}

	parts := strings.Split(versionStr, ".")
	values := []int{0, 0, 0, 0}
	for i, part := range parts {
		value, convErr := strconv.Atoi(part)
		if convErr != nil {
			return nil, fmt.Errorf("parse version part %q: %w", part, convErr)
		}
		values[i] = value
	}

	return &PackageVersion{
		Major: values[0],
		Minor: values[1],
		Patch: values[2],
		Tweak: values[3],
	}, nil
}
