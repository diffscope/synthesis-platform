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

package command

import (
	"diffscope-synthesis-platform/lib/package_manager"
	"diffscope-synthesis-platform/lib/package_manager/repository"
	"errors"
	"fmt"
	"os"
)

func RmRegistry(ids []string) error {
	if len(ids) == 0 {
		return errors.New("registry remove list is empty")
	}

	allOk := true
	for _, id := range ids {
		if err := rmRegistry(id); err != nil {
			allOk = false
			fmt.Fprintf(os.Stderr, "failed to remove registry %s: %v\n", id, err)
		}
	}

	if !allOk {
		return errors.New("one or more registries failed to be removed")
	}

	return nil
}

func rmRegistry(id string) error {
	db := package_manager.DB()
	if db == nil {
		return errors.New("package manager database is not initialized")
	}

	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()

	if _, err := repository.GetRegistryByID(tx, id); err != nil {
		return err
	}

	if err := repository.DeleteRegistryByID(tx, id); err != nil {
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	return nil
}
