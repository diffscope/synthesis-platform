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
	"net/http"

	"github.com/gin-gonic/gin"
)

type ApplicationInfo struct {
	APIVersion int `json:"api_version"`
}

func getApplicationInfo() ApplicationInfo {
	return ApplicationInfo{
		APIVersion: 1,
	}
}

func GetApplicationInfo(c *gin.Context) {
	info := getApplicationInfo()
	c.JSON(http.StatusOK, gin.H{
		"dssp": info,
	})
}
