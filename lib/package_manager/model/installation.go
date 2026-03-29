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

type Installation struct {
	Hash              string  `gorm:"column:hash;type:text;primaryKey"`
	PackageID         string  `gorm:"column:package_id;type:text;not null"`
	PackageVersion    string  `gorm:"column:package_version;type:text;not null"`
	PackageRegistryID *string `gorm:"column:package_registry_id;type:text"`
	CreatedAt         int64   `gorm:"column:created_at;not null"`

	Registry *Registry `gorm:"foreignKey:PackageRegistryID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
}
