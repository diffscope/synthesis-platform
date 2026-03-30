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

package package_manager

import (
	"diffscope-synthesis-platform/lib/package_manager/model"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

type Config struct {
	PackageDir       string
	ShouldOutputJSON bool
	NoCache          bool
	NoTTY            bool
}

var config Config

func InitializePackageManager() error {
	config.PackageDir = strings.TrimSpace(viper.GetString("package_dir"))
	config.ShouldOutputJSON = viper.GetBool("package_manager.json_output")
	config.NoCache = viper.GetBool("package_manager.no_cache")
	config.NoTTY = viper.GetBool("package_manager.no_tty")

	packageDir := config.PackageDir
	if packageDir == "" {
		return errors.New("package_dir is empty")
	}

	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		return err
	}

	dbPath := filepath.Join(packageDir, "package.db")
	// TODO: customize logger
	db_, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Discard,
	})
	if err != nil {
		return err
	}

	db = db_
	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		return err
	}

	if err := db.AutoMigrate(
		&model.Registry{},
		&model.Package{},
		&model.PackageText{},
		&model.Dependency{},
		&model.Singer{},
		&model.SingerText{},
		&model.Voice{},
		&model.VoiceText{},
		&model.Installation{},
		&model.InstallationDependency{},
	); err != nil {
		return err
	}

	return nil
}

func DB() *gorm.DB {
	if db == nil {
		panic("package manager database is not initialized")
	}
	return db
}

func GetConfig() Config {
	return config
}
