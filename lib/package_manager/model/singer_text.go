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

type SingerText struct {
	SingerPackageID         string `gorm:"column:singer_package_id;type:text;primaryKey"`
	SingerPackageVersion    string `gorm:"column:singer_package_version;type:text;primaryKey"`
	SingerPackageRegistryID string `gorm:"column:singer_package_registry_id;type:text;primaryKey"`
	SingerID                string `gorm:"column:singer_id;type:text;primaryKey"`
	Language                string `gorm:"column:language;type:text;primaryKey"`

	Name      string `gorm:"column:name;type:text"`
	AvatarURL string `gorm:"column:avatar_url;type:text"`

	Singer Singer `gorm:"foreignKey:SingerPackageID,SingerPackageVersion,SingerPackageRegistryID,SingerID;references:PackageID,PackageVersion,PackageRegistryID,ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}
