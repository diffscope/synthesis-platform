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

package api

import "encoding/json"

type SingerInfo struct {
	ID               string                        `json:"id"`
	Name             string                        `json:"name"`
	MixGroup         string                        `json:"mix_group"`
	Languages        map[string]SingerLanguageInfo `json:"languages"`
	DefaultLanguage  string                        `json:"default_language"`
	ArchSpecificInfo json.RawMessage               `json:"arch_specific_info"`
	DefaultExtra     json.RawMessage               `json:"default_extra"`
}

type SingerLanguageInfo struct {
	Name         string `json:"name"`
	DefaultLyric string `json:"default_lyric"`
}

type SingerDemoAudio struct {
	Name     string `json:"name"`
	AudioURL string `json:"audio_url"`
}
