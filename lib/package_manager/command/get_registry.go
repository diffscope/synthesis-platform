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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/viper"
)

func GetRegistry(ids []string) error {
	db := package_manager.DB()
	if db == nil {
		return errors.New("package manager database is not initialized")
	}

	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()

	var (
		registries []model.Registry
		err        error
	)

	allOk := true

	if len(ids) == 0 {
		registries, err = repository.GetAllRegistries(tx)
	} else {
		for _, id := range ids {
			registry, err := repository.GetRegistryByID(tx, id)
			if err != nil {
				allOk = false
				fmt.Fprintf(os.Stderr, "failed to get registry %s: %v\n", id, err)
				continue
			}
			registries = append(registries, *registry)
		}
	}
	if err != nil {
		return err
	}

	if viper.GetBool("package_manager.json_output") {
		printRegistriesAsJSON(registries)
	} else {
		printRegistriesAsTable(registries)
	}

	if !allOk {
		return errors.New("one or more registry entries failed to get")
	}
	return nil
}

func printRegistriesAsJSON(registries []model.Registry) error {
	out := make(map[string]interface{}, len(registries))
	for _, registry := range registries {
		out[registry.ID] = map[string]interface{}{
			"url":        registry.URL,
			"updated_at": time.Unix(registry.UpdatedAt/int64(time.Second), registry.UpdatedAt%int64(time.Second)).UTC().Format(time.RFC3339Nano),
		}
	}

	encoder := json.NewEncoder(os.Stdout)
	return encoder.Encode(out)
}

func printRegistriesAsTable(registries []model.Registry) {
	twStyle := table.StyleRounded
	twStyle.Options.SeparateRows = true
	twStyle.Format.Header = text.FormatDefault

	tw := table.NewWriter()
	tw.SetStyle(twStyle)
	tw.AppendHeader(table.Row{"ID", "URL", "Updated At"})
	for _, registry := range registries {
		tw.AppendRow(table.Row{registry.ID, registry.URL, time.Unix(registry.UpdatedAt/int64(time.Second), registry.UpdatedAt%int64(time.Second)).Format(time.UnixDate)})
	}

	_, _ = os.Stdout.WriteString(tw.Render() + "\n")
}
