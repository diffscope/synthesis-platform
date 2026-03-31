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
	"diffscope-synthesis-platform/lib/package_manager/repository"
	"diffscope-synthesis-platform/lib/utils"
	"fmt"

	"gorm.io/gorm"
)

type Dependency struct {
	ID      string
	Version string
}

type LocalPackageInfo struct {
	ID           string
	Version      string
	Dependencies []Dependency
}

type RegisteredPackageInfo struct {
	ID         string
	Version    *string
	RegistryID *string
}

type ResolveResult struct {
	// Dependencies that are already installed
	InstalledDependencies []Dependency

	// Dependencies that are not installed and cannot be resolved from database
	UnresolvedDependencies []Dependency

	// Dependencies that are not installed and can be resolved from database, but there are multiple registries in database
	AmbiguousDependencies map[Dependency][]model.Registry

	// Packages to be fetched and installed, directly required by registeredPackageInfoList
	DirectPackages []model.Package

	//Packages to be fetched and installed, as dependencies of packages required by localPackageInfoList and registeredPackageInfoList
	IndirectPackages []model.Package

	// Existing installations that will be overwritten by packages required by localPackageInfoList and registeredPackageInfoList. This is used to check with user before installation.
	OverwriteInstallations []model.Installation
}

// ResolveDependency resolves the dependencies of the packages to be installed, and returns the result.
// localPackageInfoList: the list of local packages to be installed, with their dependencies specified.
// registeredPackageInfoList: the list of packages to be installed from registry, with ID and optional Version/RegistryID.
// The dependencies of these packages will be resolved from database.
// If version in registeredPackageInfoList is nil, it means the latest version of the package will be installed.
func ResolveDependency(localPackageInfoList []LocalPackageInfo, registeredPackageInfoList []RegisteredPackageInfo) (*ResolveResult, error) {
	/*
		Inputs
		- localPackageInfoList: packages installed from local files, with their declared dependencies.
		- registeredPackageInfoList: packages to install from registries, identified by ID and optional Version/RegistryID.

		1) Validate local packages
		- If two local packages share the same ID and Version, return an error.
		- If a local package is already installed (same ID and Version), record the installation in OverwriteInstallations.

		2) Validate registry requests
		- If no package with the given ID exists in the database, add it to UnresolvedDependencies.
		- If RegistryID is provided, registry-scoped queries are used in this step to avoid ambiguity.
		- If Version is nil, select the latest version from the database (scoped to RegistryID when provided).
		- If registeredPackageInfoList contains duplicates (same ID and Version), deduplicate them.
		- If a registry request conflicts with a local package (same ID and Version), return an error.
		- If the package is already installed, record the installation in OverwriteInstallations.
		- If multiple registry entries exist for the same ID and Version, add it to AmbiguousDependencies.
		- Otherwise, add the resolved package to DirectPackages.

		3) Resolve transitive dependencies (depth-first)
		- Process local packages first, then DirectPackages.
		- For each dependency:
		  - If it has already appeared in localPackageInfoList, UnresolvedDependencies, AmbiguousDependencies,
		    DirectPackages, or IndirectPackages (same ID and Version), skip it.
		    This allows cyclic dependencies as long as all nodes in the cycle are installed.
		  - If an installation already exists, add it to InstalledDependencies.
		  - If no package exists for the ID and Version, add it to UnresolvedDependencies.
		  - If multiple registry entries exist, add it to AmbiguousDependencies.
		  - Otherwise, add it to IndirectPackages and continue resolving its dependencies.

		Notes
		- A package is uniquely identified by ID and Version; multiple versions of the same ID may coexist.
		- Installed packages are detected in the Installation table by matching ID and Version.
		- If the logic is correct, localPackageInfoList, InstalledDependencies, UnresolvedDependencies,
		  AmbiguousDependencies, DirectPackages, and IndirectPackages should not contain duplicates of ID + Version.
		- UnresolvedDependencies only contains dependencies that cannot be resolved; their parents are not marked unresolved.
		- OverwriteInstallations may overlap with localPackageInfoList, UnresolvedDependencies, AmbiguousDependencies,
		  and DirectPackages, because it represents direct installation requests that would overwrite existing installs.
		- OverwriteInstallations never overlaps with IndirectPackages. If an indirect dependency is already installed,
		  it must be listed in InstalledDependencies instead.
		- OverwriteInstallations and InstalledDependencies are distinct: only direct install requests are eligible
		  for overwrite checks (both local and registry).
	*/
	result := &ResolveResult{
		AmbiguousDependencies: make(map[Dependency][]model.Registry),
	}

	tx := DB()
	visited := make(map[string]struct{})
	localPackageIndex := make(map[string]LocalPackageInfo)
	registeredIndex := make(map[string]struct{})
	overwriteIndex := make(map[string]struct{})

	for _, localPackage := range localPackageInfoList {
		key := dependencyKey(localPackage.ID, localPackage.Version)
		if _, exists := localPackageIndex[key]; exists {
			return nil, fmt.Errorf("duplicate local package %s@%s", localPackage.ID, localPackage.Version)
		}
		localPackageIndex[key] = localPackage
		visited[key] = struct{}{}

		installations, err := repository.GetInstallationsByPackageIDVersion(tx, localPackage.ID, localPackage.Version)
		if err != nil {
			return nil, err
		}
		addOverwriteInstallations(result, installations, overwriteIndex)
	}

	for _, registeredPackage := range registeredPackageInfoList {
		version, found, err := resolveRegisteredVersion(tx, registeredPackage)
		if err != nil {
			return nil, err
		}
		if !found {
			dependency := Dependency{ID: registeredPackage.ID, Version: ""}
			addUnresolvedDependency(result, dependency, visited)
			continue
		}

		key := dependencyKey(registeredPackage.ID, version)
		if _, exists := localPackageIndex[key]; exists {
			return nil, fmt.Errorf("package %s@%s is already provided by local packages", registeredPackage.ID, version)
		}
		if _, exists := registeredIndex[key]; exists {
			continue
		}
		registeredIndex[key] = struct{}{}

		packages, err := getRegisteredPackages(tx, registeredPackage.ID, version, registeredPackage.RegistryID)
		if err != nil {
			return nil, err
		}
		dependency := Dependency{ID: registeredPackage.ID, Version: version}
		if len(packages) == 0 {
			addUnresolvedDependency(result, dependency, visited)
			continue
		}
		if len(packages) > 1 {
			registries, err := repository.GetRegistriesForPackages(tx, packages)
			if err != nil {
				return nil, err
			}
			result.AmbiguousDependencies[dependency] = registries
			visited[key] = struct{}{}
			continue
		}

		installations, err := repository.GetInstallationsByPackageIDVersion(tx, registeredPackage.ID, version)
		if err != nil {
			return nil, err
		}
		addOverwriteInstallations(result, installations, overwriteIndex)

		result.DirectPackages = append(result.DirectPackages, packages[0])
		visited[key] = struct{}{}
	}

	resolveDependency := func(dep Dependency) error { return nil }
	resolveDependency = func(dep Dependency) error {
		key := dependencyKey(dep.ID, dep.Version)
		if _, exists := visited[key]; exists {
			return nil
		}
		visited[key] = struct{}{}

		installations, err := repository.GetInstallationsByPackageIDVersion(tx, dep.ID, dep.Version)
		if err != nil {
			return err
		}
		if len(installations) > 0 {
			result.InstalledDependencies = append(result.InstalledDependencies, dep)
			return nil
		}

		packages, err := repository.GetPackagesByIDVersion(tx, dep.ID, dep.Version)
		if err != nil {
			return err
		}
		if len(packages) == 0 {
			result.UnresolvedDependencies = append(result.UnresolvedDependencies, dep)
			return nil
		}
		if len(packages) > 1 {
			registries, err := repository.GetRegistriesForPackages(tx, packages)
			if err != nil {
				return err
			}
			result.AmbiguousDependencies[dep] = registries
			return nil
		}

		pkg := packages[0]
		result.IndirectPackages = append(result.IndirectPackages, pkg)

		dependencies, err := repository.GetDependenciesByPackage(tx, pkg)
		if err != nil {
			return err
		}
		for _, dependency := range dependencies {
			if err := resolveDependency(Dependency{ID: dependency.ID, Version: dependency.Version}); err != nil {
				return err
			}
		}

		return nil
	}

	for _, localPackage := range localPackageInfoList {
		for _, dependency := range localPackage.Dependencies {
			if err := resolveDependency(dependency); err != nil {
				return nil, err
			}
		}
	}

	for _, pkg := range result.DirectPackages {
		dependencies, err := repository.GetDependenciesByPackage(tx, pkg)
		if err != nil {
			return nil, err
		}
		for _, dependency := range dependencies {
			if err := resolveDependency(Dependency{ID: dependency.ID, Version: dependency.Version}); err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}

func dependencyKey(id string, version string) string {
	return fmt.Sprintf("%s@%s", id, version)
}

func addOverwriteInstallations(result *ResolveResult, installations []model.Installation, overwriteIndex map[string]struct{}) {
	for _, installation := range installations {
		if _, exists := overwriteIndex[installation.Hash]; exists {
			continue
		}
		overwriteIndex[installation.Hash] = struct{}{}
		result.OverwriteInstallations = append(result.OverwriteInstallations, installation)
	}
}

func addUnresolvedDependency(result *ResolveResult, dependency Dependency, visited map[string]struct{}) {
	key := dependencyKey(dependency.ID, dependency.Version)
	if _, exists := visited[key]; exists {
		return
	}
	visited[key] = struct{}{}
	result.UnresolvedDependencies = append(result.UnresolvedDependencies, dependency)
}

func resolveRegisteredVersion(tx *gorm.DB, registeredPackage RegisteredPackageInfo) (string, bool, error) {
	if registeredPackage.Version != nil {
		return *registeredPackage.Version, true, nil
	}

	latestVersion, found, err := findLatestVersionByID(tx, registeredPackage.ID, registeredPackage.RegistryID)
	if err != nil {
		return "", false, err
	}
	if !found {
		return "", false, nil
	}

	return latestVersion, true, nil
}

func getRegisteredPackages(tx *gorm.DB, id string, version string, registryID *string) ([]model.Package, error) {
	if registryID == nil {
		return repository.GetPackagesByIDVersion(tx, id, version)
	}

	return repository.GetPackagesByIDVersionRegistry(tx, id, version, *registryID)
}

func findLatestVersionByID(tx *gorm.DB, id string, registryID *string) (string, bool, error) {
	var packages []model.Package
	var err error
	if registryID == nil {
		packages, err = repository.GetPackagesByID(tx, id)
	} else {
		packages, err = repository.GetPackagesByIDRegistry(tx, id, *registryID)
	}
	if err != nil {
		return "", false, err
	}
	if len(packages) == 0 {
		return "", false, nil
	}

	var best *utils.PackageVersion
	var bestVersion string
	for _, pkg := range packages {
		parsed, err := utils.ParsePackageVersion(pkg.Version)
		if err != nil {
			return "", false, fmt.Errorf("parse version %s for package %s: %w", pkg.Version, pkg.ID, err)
		}
		if best == nil || comparePackageVersion(parsed, best) > 0 {
			best = parsed
			bestVersion = pkg.Version
		}
	}

	return bestVersion, true, nil
}

func comparePackageVersion(a *utils.PackageVersion, b *utils.PackageVersion) int {
	if a.Major != b.Major {
		return compareInts(a.Major, b.Major)
	}
	if a.Minor != b.Minor {
		return compareInts(a.Minor, b.Minor)
	}
	if a.Patch != b.Patch {
		return compareInts(a.Patch, b.Patch)
	}
	return compareInts(a.Tweak, b.Tweak)
}

func compareInts(a int, b int) int {
	if a > b {
		return 1
	}
	if a < b {
		return -1
	}
	return 0
}
