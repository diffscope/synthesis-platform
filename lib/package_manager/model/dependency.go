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

type Dependency struct {
	PackageID         string `gorm:"column:package_id;type:text;not null"`
	PackageVersion    string `gorm:"column:package_version;type:text;not null"`
	PackageRegistryID string `gorm:"column:package_registry_id;type:text;not null"`

	ID      string `gorm:"column:id;type:text;not null"`
	Version string `gorm:"column:version;type:text;not null"`

	Package Package `gorm:"foreignKey:PackageID,PackageVersion,PackageRegistryID;references:ID,Version,RegistryID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}
