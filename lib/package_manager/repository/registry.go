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

package repository

import (
	"diffscope-synthesis-platform/lib/package_manager/model"

	"gorm.io/gorm"
)

func GetAllRegistries(tx *gorm.DB) ([]model.Registry, error) {
	registries := make([]model.Registry, 0)
	if err := tx.Order("id ASC").Find(&registries).Error; err != nil {
		return nil, err
	}

	return registries, nil
}

func GetRegistryByID(tx *gorm.DB, id string) (*model.Registry, error) {
	var registry model.Registry
	if err := tx.Where("id = ?", id).Take(&registry).Error; err != nil {
		return nil, err
	}

	return &registry, nil
}

func CreateRegistry(tx *gorm.DB, registry *model.Registry) error {
	return tx.Create(registry).Error
}

func UpdateRegistry(tx *gorm.DB, registry *model.Registry) error {
	return tx.Model(&model.Registry{}).
		Where("id = ?", registry.ID).
		Updates(map[string]interface{}{
			"url":        registry.URL,
			"updated_at": registry.UpdatedAt,
		}).Error
}

func DeleteRegistryByID(tx *gorm.DB, id string) error {
	return tx.Where("id = ?", id).Delete(&model.Registry{}).Error
}
