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

type Package struct {
	ID         string `gorm:"column:id;type:text;primaryKey"`
	Version    string `gorm:"column:version;type:text;primaryKey"`
	RegistryID string `gorm:"column:registry_id;type:text;primaryKey"`

	DownloadURL    string `gorm:"column:download_url;type:text;not null"`
	DownloadSHA512 string `gorm:"column:download_sha512;type:text;not null"`
	UpdatedAt      int64  `gorm:"column:updated_at;not null"`

	Registry     Registry      `gorm:"foreignKey:RegistryID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Texts        []PackageText `gorm:"foreignKey:PackageID,PackageVersion,PackageRegistryID;references:ID,Version,RegistryID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Dependencies []Dependency  `gorm:"foreignKey:PackageID,PackageVersion,PackageRegistryID;references:ID,Version,RegistryID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Singers      []Singer      `gorm:"foreignKey:PackageID,PackageVersion,PackageRegistryID;references:ID,Version,RegistryID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}
