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

package server

import (
	"diffscope-synthesis-platform/lib/server/controller"
	"diffscope-synthesis-platform/native"
	"fmt"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func StartLanguageService() error {
	ep, err := native.ExecutionProviderTypeFromString(viper.GetString("execution_provider.type"))
	if err != nil {
		return err
	}
	deviceIndex := viper.GetInt("execution_provider.device_index")
	if nativeErr := native.LanguageServiceInitialize(ep, deviceIndex); nativeErr.Swigcptr() != 0 {
		defer native.DeleteLanguageServiceInitializationError(nativeErr)
		return fmt.Errorf("failed to initialize native language service: %s", nativeErr.Error())
	}
	return nil
}

func StartRouter() error {
	router := gin.Default()
	router.Use(gzip.Gzip(gzip.DefaultCompression))

	router.GET("/api/info", controller.GetApplicationInfo)
	router.POST("/api/language", controller.PostLanguage)

	host := viper.GetString("host")
	port := viper.GetInt("port")

	return router.Run(fmt.Sprintf("%s:%d", host, port))
}

func StartServer() error {
	defaultDevice := native.ExecutionProviderInfoGetDefaultDevice()
	viper.SetDefault("execution_provider.type", defaultDevice.Type().String())
	viper.SetDefault("execution_provider.device_index", defaultDevice.Index())

	if err := StartLanguageService(); err != nil {
		return err
	}
	if err := StartRouter(); err != nil {
		return err
	}
	return nil
}
