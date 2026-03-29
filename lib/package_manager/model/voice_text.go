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

package model

type VoiceText struct {
	VoiceSingerPackageID         string `gorm:"column:voice_singer_package_id;type:text;primaryKey"`
	VoiceSingerPackageVersion    string `gorm:"column:voice_singer_package_version;type:text;primaryKey"`
	VoiceSingerPackageRegistryID string `gorm:"column:voice_singer_package_registry_id;type:text;primaryKey"`
	VoiceSingerID                string `gorm:"column:voice_singer_id;type:text;primaryKey"`
	VoiceID                      string `gorm:"column:voice_id;type:text;primaryKey"`
	Language                     string `gorm:"column:language;type:text;primaryKey"`

	Name         string `gorm:"column:name;type:text"`
	DemoAudioURL string `gorm:"column:demo_audio_url;type:text"`

	Voice Voice `gorm:"foreignKey:VoiceSingerPackageID,VoiceSingerPackageVersion,VoiceSingerPackageRegistryID,VoiceSingerID,VoiceID;references:SingerPackageID,SingerPackageVersion,SingerPackageRegistryID,SingerID,ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}
