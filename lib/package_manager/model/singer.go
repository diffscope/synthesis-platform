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

type Singer struct {
	PackageID         string `gorm:"column:package_id;type:text;primaryKey"`
	PackageVersion    string `gorm:"column:package_version;type:text;primaryKey"`
	PackageRegistryID string `gorm:"column:package_registry_id;type:text;primaryKey"`
	ID                string `gorm:"column:id;type:text;primaryKey"`

	Package Package      `gorm:"foreignKey:PackageID,PackageVersion,PackageRegistryID;references:ID,Version,RegistryID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Texts   []SingerText `gorm:"foreignKey:SingerPackageID,SingerPackageVersion,SingerPackageRegistryID,SingerID;references:PackageID,PackageVersion,PackageRegistryID,ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Voices  []Voice      `gorm:"foreignKey:SingerPackageID,SingerPackageVersion,SingerPackageRegistryID,SingerID;references:PackageID,PackageVersion,PackageRegistryID,ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}
