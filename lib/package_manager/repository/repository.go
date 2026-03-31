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

func GetPackagesByID(tx *gorm.DB, id string) ([]model.Package, error) {
	packages := make([]model.Package, 0)
	if err := tx.Where("id = ?", id).Find(&packages).Error; err != nil {
		return nil, err
	}

	return packages, nil
}

func GetPackagesByIDVersion(tx *gorm.DB, id string, version string) ([]model.Package, error) {
	packages := make([]model.Package, 0)
	if err := tx.Where("id = ? AND version = ?", id, version).Find(&packages).Error; err != nil {
		return nil, err
	}

	return packages, nil
}

func GetPackagesByIDRegistry(tx *gorm.DB, id string, registryID string) ([]model.Package, error) {
	packages := make([]model.Package, 0)
	if err := tx.Where("id = ? AND registry_id = ?", id, registryID).Find(&packages).Error; err != nil {
		return nil, err
	}

	return packages, nil
}

func GetPackagesByIDVersionRegistry(tx *gorm.DB, id string, version string, registryID string) ([]model.Package, error) {
	packages := make([]model.Package, 0)
	if err := tx.Where("id = ? AND version = ? AND registry_id = ?", id, version, registryID).Find(&packages).Error; err != nil {
		return nil, err
	}

	return packages, nil
}

func GetDependenciesByPackage(tx *gorm.DB, pkg model.Package) ([]model.Dependency, error) {
	dependencies := make([]model.Dependency, 0)
	if err := tx.Where("package_id = ? AND package_version = ? AND package_registry_id = ?", pkg.ID, pkg.Version, pkg.RegistryID).
		Find(&dependencies).Error; err != nil {
		return nil, err
	}

	return dependencies, nil
}

func GetInstallationsByPackageIDVersion(tx *gorm.DB, id string, version string) ([]model.Installation, error) {
	installations := make([]model.Installation, 0)
	if err := tx.Where("package_id = ? AND package_version = ?", id, version).
		Find(&installations).Error; err != nil {
		return nil, err
	}

	return installations, nil
}

func GetRegistriesForPackages(tx *gorm.DB, packages []model.Package) ([]model.Registry, error) {
	registryIDSet := make(map[string]struct{})
	registryIDs := make([]string, 0, len(packages))
	for _, pkg := range packages {
		if _, exists := registryIDSet[pkg.RegistryID]; exists {
			continue
		}
		registryIDSet[pkg.RegistryID] = struct{}{}
		registryIDs = append(registryIDs, pkg.RegistryID)
	}

	if len(registryIDs) == 0 {
		return []model.Registry{}, nil
	}

	registries := make([]model.Registry, 0)
	if err := tx.Where("id IN ?", registryIDs).Find(&registries).Error; err != nil {
		return nil, err
	}

	return registries, nil
}
