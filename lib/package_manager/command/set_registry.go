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
	"diffscope-synthesis-platform/lib/package_manager/model"
	"diffscope-synthesis-platform/lib/package_manager/repository"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
)

type RegistrySetEntry struct {
	ID  string
	URL string
}

func SetRegistry(entries []RegistrySetEntry) error {
	if len(entries) == 0 {
		return errors.New("registry set list is empty")
	}

	allOk := true
	for _, entry := range entries {
		if err := setRegistry(entry.ID, entry.URL); err != nil {
			allOk = false
			fmt.Fprintf(os.Stderr, "failed to set registry %s=%s: %v\n", entry.ID, entry.URL, err)
		}
	}

	if !allOk {
		return errors.New("one or more registry entries failed to set")
	}

	return nil
}

func setRegistry(id string, url string) error {
	re := regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)

	id = strings.TrimSpace(id)
	url = strings.TrimSpace(url)

	if !re.MatchString(id) {
		return errors.New("registry id is invalid: id should not be empty and should only contain letters, digits, underscores and hyphens")
	}
	if url == "" {
		return errors.New("registry url is empty")
	}

	db := package_manager.DB()

	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()

	now := time.Now().UnixNano()
	registry, err := repository.GetRegistryByID(tx, id)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		registry = &model.Registry{
			ID:        id,
			URL:       url,
			UpdatedAt: now,
		}
		if err := repository.CreateRegistry(tx, registry); err != nil {
			return err
		}
	} else {
		registry.URL = url
		registry.UpdatedAt = now
		if err := repository.UpdateRegistry(tx, registry); err != nil {
			return err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	return nil
}
