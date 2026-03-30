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
	"diffscope-synthesis-platform/lib/utils"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-playground/validator/v10"
	"golang.org/x/term"
	"gorm.io/gorm"
)

var (
	versionPattern   = regexp.MustCompile(`^\d{1,4}(?:\.\d{1,4}){0,3}$`)
	singerIDPattern  = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	packageIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+(?:/[A-Za-z0-9_-]+)*$`)
)

type Registry struct {
	Packages []Package `json:"packages" validate:"required,dive"`
}

type Package struct {
	ID       string           `json:"id" validate:"required,package_id"`
	Versions []PackageVersion `json:"versions" validate:"required,dive"`
}

type PackageVersion struct {
	Version        string       `json:"version" validate:"required,version"`
	Name           DisplayText  `json:"name" validate:"required,display_text"`
	Vendor         DisplayText  `json:"vendor" validate:"required,display_text"`
	Description    DisplayText  `json:"description" validate:"required,display_text"`
	DownloadURL    string       `json:"download_url" validate:"required,url"`
	DownloadSHA512 string       `json:"download_sha512" validate:"required,sha512"`
	Dependencies   []Dependency `json:"dependencies" validate:"required,dive"`
	Singers        []Singer     `json:"singers" validate:"required,dive"`
}

type Dependency struct {
	ID      string `json:"id" validate:"required,package_id"`
	Version string `json:"version" validate:"required,version"`
}

type Singer struct {
	ID        string      `json:"id" validate:"required,singer_id"`
	Name      DisplayText `json:"name" validate:"required,display_text,dive,required"`
	AvatarURL DisplayURL  `json:"avatar_url" validate:"required,display_text,dive,required,url"`
	Voices    []Voice     `json:"voices" validate:"required,dive"`
}

type Voice struct {
	ID           string      `json:"id" validate:"required,singer_id"`
	Name         DisplayText `json:"name" validate:"required,display_text,dive,required"`
	DemoAudioURL DisplayURL  `json:"demo_audio_url" validate:"required,display_text,dive,required,url"`
}

type DisplayText map[string]string

type DisplayURL map[string]string

func validateRegistry(registry Registry) error {
	v := validator.New()

	v.RegisterValidation("version", func(fl validator.FieldLevel) bool {
		return versionPattern.MatchString(fl.Field().String())
	})

	v.RegisterValidation("singer_id", func(fl validator.FieldLevel) bool {
		return singerIDPattern.MatchString(fl.Field().String())
	})

	v.RegisterValidation("package_id", func(fl validator.FieldLevel) bool {
		return packageIDPattern.MatchString(fl.Field().String())
	})

	v.RegisterValidation("display_text", func(fl validator.FieldLevel) bool {
		field := fl.Field()
		if field.Kind() != reflect.Map || field.IsNil() {
			return false
		}

		underscore := field.MapIndex(reflect.ValueOf("_"))
		if !underscore.IsValid() {
			return false
		}

		return underscore.Kind() == reflect.String
	})

	return v.Struct(registry)
}

func Update(ids []string) error {
	config := package_manager.GetConfig()
	packageDir := config.PackageDir
	noCache := config.NoCache

	db := package_manager.DB()

	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()

	var registries []model.Registry
	allOk := true

	var err error

	if len(ids) == 0 {
		registries, err = repository.GetAllRegistries(tx)
	} else {
		for _, id := range ids {
			registry, getErr := repository.GetRegistryByID(tx, id)
			if getErr != nil {
				allOk = false
				continue
			}
			registries = append(registries, *registry)
		}
	}
	if err != nil {
		return err
	}

	notifyRegistryList(registries)
	defer notifyShutdown()

	regCacheDir := filepath.Join(packageDir, ".regcache")
	for _, registry := range registries {
		notifyNextRegistry()

		if err := utils.DownloadFromHttp(
			registry.URL,
			regCacheDir,
			registry.ID,
			noCache,
			notifyDownloadProgress,
		); err != nil {
			notifyDownloadError(err.Error())
			allOk = false
			continue
		}

		registryDataPath := filepath.Join(regCacheDir, registry.ID)
		parsedRegistry, err := readAndValidateRegistryData(registryDataPath)
		if err != nil {
			notifyValidationError(err.Error())
			allOk = false
			continue
		}

		var (
			removed int
			added   int
		)
		tx.Transaction(func(tx2 *gorm.DB) error {
			removed, added, err = replaceRegistryPackages(tx2, registry.ID, parsedRegistry)
			return err
		})
		if err != nil {
			notifyRepositoryError(err.Error())
			allOk = false
			continue
		}

		notifyResult(removed, added)
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	if !allOk {
		return errors.New("one or more registry entries failed to update")
	}

	return nil
}

func readAndValidateRegistryData(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var registry Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, err
	}

	if err := validateRegistry(registry); err != nil {
		return nil, err
	}

	return &registry, nil
}

func replaceRegistryPackages(tx *gorm.DB, registryID string, registry *Registry) (int, int, error) {
	deleteResult := tx.Where("registry_id = ?", registryID).Delete(&model.Package{})
	if deleteResult.Error != nil {
		return 0, 0, deleteResult.Error
	}
	removed := int(deleteResult.RowsAffected)

	added := 0
	now := time.Now().UnixNano()
	for _, pkg := range registry.Packages {
		for _, version := range pkg.Versions {
			pkgModel := model.Package{
				ID:             pkg.ID,
				Version:        version.Version,
				RegistryID:     registryID,
				DownloadURL:    version.DownloadURL,
				DownloadSHA512: version.DownloadSHA512,
				UpdatedAt:      now,
			}
			if err := tx.Create(&pkgModel).Error; err != nil {
				return 0, 0, err
			}
			added++

			for _, dep := range version.Dependencies {
				depModel := model.Dependency{
					PackageID:         pkg.ID,
					PackageVersion:    version.Version,
					PackageRegistryID: registryID,
					ID:                dep.ID,
					Version:           dep.Version,
				}
				if err := tx.Create(&depModel).Error; err != nil {
					return 0, 0, err
				}
			}

			for _, language := range collectLanguageKeys(version.Name, version.Vendor, version.Description) {
				packageText := model.PackageText{
					PackageID:         pkg.ID,
					PackageVersion:    version.Version,
					PackageRegistryID: registryID,
					Language:          language,
					Name:              version.Name[language],
					Vendor:            version.Vendor[language],
					Description:       version.Description[language],
				}
				if err := tx.Create(&packageText).Error; err != nil {
					return 0, 0, err
				}
			}

			for _, singer := range version.Singers {
				singerModel := model.Singer{
					PackageID:         pkg.ID,
					PackageVersion:    version.Version,
					PackageRegistryID: registryID,
					ID:                singer.ID,
				}
				if err := tx.Create(&singerModel).Error; err != nil {
					return 0, 0, err
				}

				for _, language := range collectLanguageKeys(singer.Name, singer.AvatarURL) {
					singerText := model.SingerText{
						SingerPackageID:         pkg.ID,
						SingerPackageVersion:    version.Version,
						SingerPackageRegistryID: registryID,
						SingerID:                singer.ID,
						Language:                language,
						Name:                    singer.Name[language],
						AvatarURL:               singer.AvatarURL[language],
					}
					if err := tx.Create(&singerText).Error; err != nil {
						return 0, 0, err
					}
				}

				for _, voice := range singer.Voices {
					voiceModel := model.Voice{
						SingerPackageID:         pkg.ID,
						SingerPackageVersion:    version.Version,
						SingerPackageRegistryID: registryID,
						SingerID:                singer.ID,
						ID:                      voice.ID,
					}
					if err := tx.Create(&voiceModel).Error; err != nil {
						return 0, 0, err
					}

					for _, language := range collectLanguageKeys(voice.Name, voice.DemoAudioURL) {
						voiceText := model.VoiceText{
							VoiceSingerPackageID:         pkg.ID,
							VoiceSingerPackageVersion:    version.Version,
							VoiceSingerPackageRegistryID: registryID,
							VoiceSingerID:                singer.ID,
							VoiceID:                      voice.ID,
							Language:                     language,
							Name:                         voice.Name[language],
							DemoAudioURL:                 voice.DemoAudioURL[language],
						}
						if err := tx.Create(&voiceText).Error; err != nil {
							return 0, 0, err
						}
					}
				}
			}
		}
	}

	return removed, added, nil
}

func collectLanguageKeys(maps ...map[string]string) []string {
	seen := make(map[string]struct{})
	keys := make([]string, 0)

	for _, data := range maps {
		for language := range data {
			if _, ok := seen[language]; ok {
				continue
			}
			seen[language] = struct{}{}
			keys = append(keys, language)
		}
	}

	return keys
}

// Notification renderers

var (
	currentRegistryList  []model.Registry
	currentRegistryIndex int              = -1
	notifyRenderer       notifierRenderer = &plainTextNotifier{}
)

type notifierRenderer interface {
	notifyRegistryList(registries []model.Registry)
	notifyNextRegistry(index int, registry model.Registry)
	notifyDownloadProgress(index int, registry model.Registry, totalBytes int64, downloadedBytes int64, remaining time.Duration, speedBytesPerSecond float64)
	notifyDownloadError(index int, registry model.Registry, message string)
	notifyValidationError(index int, registry model.Registry, message string)
	notifyRepositoryError(index int, registry model.Registry, message string)
	notifyResult(index int, registry model.Registry, removed int, added int)
	shutdown() error
}

type plainTextNotifier struct{}

func (n *plainTextNotifier) notifyRegistryList(registries []model.Registry) {
	fmt.Printf("Updating %d registries\n", len(registries))
}

func (n *plainTextNotifier) notifyNextRegistry(index int, registry model.Registry) {
	fmt.Printf("[%d/%d] Updating %s\n", index+1, len(currentRegistryList), registry.ID)
}

func (n *plainTextNotifier) notifyDownloadProgress(index int, registry model.Registry, totalBytes int64, downloadedBytes int64, remaining time.Duration, speedBytesPerSecond float64) {
	fmt.Printf("[%d/%d] Downloading %s from %s\n", index+1, len(currentRegistryList), registry.ID, registry.URL)
	fmt.Printf("%5.1f%% %s/%s %s\n", percentFloat(totalBytes, downloadedBytes)*100, formatBytes(downloadedBytes), formatBytes(totalBytes), formatRemaining(remaining))
}

func (n *plainTextNotifier) notifyDownloadError(index int, registry model.Registry, message string) {
	fmt.Printf("[%d/%d] Failed to download %s from %s:\n", index+1, len(currentRegistryList), registry.ID, registry.URL)
	fmt.Printf("%s\n", message)
}

func (n *plainTextNotifier) notifyValidationError(index int, registry model.Registry, message string) {
	fmt.Printf("[%d/%d] Failed to validate %s downloaded from %s:\n", index+1, len(currentRegistryList), registry.ID, registry.URL)
	fmt.Printf("%s\n", message)
}

func (n *plainTextNotifier) notifyRepositoryError(index int, registry model.Registry, message string) {
	fmt.Printf("[%d/%d] Failed to update local package entry database when updating %s:\n", index+1, len(currentRegistryList), registry.ID)
	fmt.Printf("%s\n", message)
}

func (n *plainTextNotifier) notifyResult(index int, registry model.Registry, removed int, added int) {
	fmt.Printf("[%d/%d] %s update complete. %d entries added, %d entries removed.\n", index+1, len(currentRegistryList), registry.ID, added, removed)
}

func (n *plainTextNotifier) shutdown() error {
	return nil
}

type tuiRowMode int

const (
	tuiRowModeProgress tuiRowMode = iota
	tuiRowModeError
)

type tuiRegistryRow struct {
	visible    bool
	line1      string
	mode       tuiRowMode
	line1State tuiLine1State
	line2Error string

	percent             float64
	totalBytes          int64
	downloadedBytes     int64
	remaining           time.Duration
	speedBytesPerSecond float64
}

type tuiLine1State int

const (
	tuiLine1StateNormal tuiLine1State = iota
	tuiLine1StateDone
	tuiLine1StateError
)

type tuiUpdateModel struct {
	width    int
	rows     []tuiRegistryRow
	progress progress.Model
}

type tuiRegistryListMsg struct {
	registries []model.Registry
}

type tuiNextRegistryMsg struct {
	index    int
	registry model.Registry
}

type tuiDownloadProgressMsg struct {
	index               int
	registry            model.Registry
	totalBytes          int64
	downloadedBytes     int64
	remaining           time.Duration
	speedBytesPerSecond float64
}

type tuiErrorKind string

const (
	tuiErrorKindDownload   tuiErrorKind = "Failed to download"
	tuiErrorKindValidation tuiErrorKind = "Failed to validate"
	tuiErrorKindRepository tuiErrorKind = "Failed to update local package entry database"
)

type tuiErrorMsg struct {
	index    int
	registry model.Registry
	kind     tuiErrorKind
	message  string
}

type tuiResultMsg struct {
	index   int
	removed int
	added   int
}

type tuiShutdownMsg struct{}

var (
	tuiDoneStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	tuiErrorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	tuiProgressStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	tuiSpeedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	tuiRemainingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
)

func newTUIUpdateModel() tuiUpdateModel {
	bar := progress.New(progress.WithoutPercentage())
	bar.ShowPercentage = false
	bar.FullColor = "#5566FF"

	return tuiUpdateModel{
		rows:     make([]tuiRegistryRow, 0),
		progress: bar,
	}
}

func (m tuiUpdateModel) Init() tea.Cmd {
	return nil
}

func (m tuiUpdateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = v.Width
		return m, nil
	case tuiRegistryListMsg:
		m.rows = make([]tuiRegistryRow, len(v.registries))
		return m, nil
	case tuiNextRegistryMsg:
		if !m.ensureIndex(v.index) {
			return m, nil
		}
		m.rows[v.index].visible = true
		m.rows[v.index].mode = tuiRowModeProgress
		m.rows[v.index].line1State = tuiLine1StateNormal
		m.rows[v.index].line1 = fmt.Sprintf("[%d/%d] Updating %s", v.index+1, len(m.rows), v.registry.ID)
		m.rows[v.index].line2Error = ""
		m.rows[v.index].percent = 0
		m.rows[v.index].totalBytes = 0
		m.rows[v.index].downloadedBytes = 0
		m.rows[v.index].remaining = 0
		m.rows[v.index].speedBytesPerSecond = 0
		return m, nil
	case tuiDownloadProgressMsg:
		if !m.ensureIndex(v.index) {
			return m, nil
		}
		row := &m.rows[v.index]
		row.visible = true
		row.mode = tuiRowModeProgress
		row.line1State = tuiLine1StateNormal
		row.line1 = fmt.Sprintf("[%d/%d] Downloading %s from %s", v.index+1, len(m.rows), v.registry.ID, v.registry.URL)
		row.line2Error = ""
		row.percent = percentFloat(v.totalBytes, v.downloadedBytes)
		row.totalBytes = v.totalBytes
		row.downloadedBytes = v.downloadedBytes
		row.remaining = v.remaining
		row.speedBytesPerSecond = v.speedBytesPerSecond
		return m, nil
	case tuiErrorMsg:
		if !m.ensureIndex(v.index) {
			return m, nil
		}
		row := &m.rows[v.index]
		row.visible = true
		row.mode = tuiRowModeError
		row.line1State = tuiLine1StateError
		switch v.kind {
		case tuiErrorKindDownload:
			row.line1 = fmt.Sprintf("[%d/%d] Failed to download %s from %s:", v.index+1, len(m.rows), v.registry.ID, v.registry.URL)
		case tuiErrorKindValidation:
			row.line1 = fmt.Sprintf("[%d/%d] Failed to validate %s downloaded from %s:", v.index+1, len(m.rows), v.registry.ID, v.registry.URL)
		case tuiErrorKindRepository:
			row.line1 = fmt.Sprintf("[%d/%d] Failed to update local package entry database when updating %s:", v.index+1, len(m.rows), v.registry.ID)
		}
		row.line2Error = v.message
		return m, nil
	case tuiResultMsg:
		if !m.ensureIndex(v.index) {
			return m, nil
		}
		row := &m.rows[v.index]
		registry := currentRegistryList[v.index]
		row.visible = true
		row.mode = tuiRowModeProgress
		row.line1State = tuiLine1StateDone
		row.line1 = fmt.Sprintf("[%d/%d] %s update complete. %d entries added, %d entries removed.", v.index+1, len(m.rows), registry.ID, v.added, v.removed)
		if row.totalBytes > 0 {
			row.downloadedBytes = row.totalBytes
			row.percent = 1
		}
		row.remaining = 0
		return m, nil
	case tuiShutdownMsg:
		return m, tea.Quit
	}

	return m, nil
}

func (m tuiUpdateModel) ensureIndex(index int) bool {
	return index >= 0 && index < len(m.rows)
}

func (m tuiUpdateModel) View() string {
	if len(m.rows) == 0 {
		return ""
	}

	out := ""
	for _, row := range m.rows {
		if !row.visible {
			continue
		}

		line1 := row.line1
		switch row.line1State {
		case tuiLine1StateDone:
			line1 = tuiDoneStyle.Render(line1)
		case tuiLine1StateError:
			line1 = tuiErrorStyle.Render(line1)
		}

		out += line1 + "\n"
		if row.mode == tuiRowModeError {
			out += row.line2Error + "\n"
			continue
		}
		out += m.renderProgressLine(row) + "\n"
	}

	return out
}

func (m tuiUpdateModel) renderProgressLine(row tuiRegistryRow) string {
	percentText := fmt.Sprintf("%5.1f%%", row.percent*100)
	progressText := fmt.Sprintf("%s/%s", formatBytes(row.downloadedBytes), formatBytes(row.totalBytes))
	speedValue := row.speedBytesPerSecond
	if speedValue < 0 {
		speedValue = 0
	}
	if speedValue > float64(math.MaxInt64) {
		speedValue = float64(math.MaxInt64)
	}
	speedText := fmt.Sprintf("%s/s", formatBytes(int64(speedValue)))
	remainingText := formatRemaining(row.remaining)
	rightText := fmt.Sprintf(
		"%s %s %s %s",
		percentText,
		tuiProgressStyle.Render(progressText),
		tuiSpeedStyle.Render(speedText),
		tuiRemainingStyle.Render(remainingText),
	)

	rightWidth := lipgloss.Width(rightText)
	barWidth := m.width - rightWidth - 1
	if barWidth < 4 {
		barWidth = 4
	}

	p := m.progress
	p.Width = barWidth
	return p.ViewAs(row.percent) + " " + rightText
}

type tuiNotifier struct {
	p      *tea.Program
	doneCh chan struct{}
	errMu  sync.Mutex
	runErr error
}

func newTUINotifier() *tuiNotifier {
	model := newTUIUpdateModel()
	p := tea.NewProgram(model)
	n := &tuiNotifier{
		p:      p,
		doneCh: make(chan struct{}),
	}

	go func() {
		_, err := p.Run()
		n.errMu.Lock()
		n.runErr = err
		n.errMu.Unlock()
		close(n.doneCh)
	}()

	return n
}

func (n *tuiNotifier) notifyRegistryList(registries []model.Registry) {
	n.p.Send(tuiRegistryListMsg{registries: registries})
}

func (n *tuiNotifier) notifyNextRegistry(index int, registry model.Registry) {
	n.p.Send(tuiNextRegistryMsg{index: index, registry: registry})
}

func (n *tuiNotifier) notifyDownloadProgress(index int, registry model.Registry, totalBytes int64, downloadedBytes int64, remaining time.Duration, speedBytesPerSecond float64) {
	n.p.Send(tuiDownloadProgressMsg{
		index:               index,
		registry:            registry,
		totalBytes:          totalBytes,
		downloadedBytes:     downloadedBytes,
		remaining:           remaining,
		speedBytesPerSecond: speedBytesPerSecond,
	})
}

func (n *tuiNotifier) notifyDownloadError(index int, registry model.Registry, message string) {
	n.p.Send(tuiErrorMsg{index: index, registry: registry, kind: tuiErrorKindDownload, message: message})
}

func (n *tuiNotifier) notifyValidationError(index int, registry model.Registry, message string) {
	n.p.Send(tuiErrorMsg{index: index, registry: registry, kind: tuiErrorKindValidation, message: message})
}

func (n *tuiNotifier) notifyRepositoryError(index int, registry model.Registry, message string) {
	n.p.Send(tuiErrorMsg{index: index, registry: registry, kind: tuiErrorKindRepository, message: message})
}

func (n *tuiNotifier) notifyResult(index int, registry model.Registry, removed int, added int) {
	n.p.Send(tuiResultMsg{index: index, removed: removed, added: added})
}

func (n *tuiNotifier) shutdown() error {
	n.p.Send(tuiShutdownMsg{})
	<-n.doneCh

	n.errMu.Lock()
	defer n.errMu.Unlock()
	return n.runErr
}

func shouldUsePlainTextOutput() bool {
	config := package_manager.GetConfig()
	if config.NoTTY {
		return true
	}
	return !term.IsTerminal(int(os.Stdout.Fd()))
}

func notifyShutdown() {
	if err := notifyRenderer.shutdown(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to close TUI renderer: %v\n", err)
	}
	notifyRenderer = &plainTextNotifier{}
}

func formatBytes(v int64) string {
	if v < 0 {
		v = 0
	}
	const unit = 1024
	if v < unit {
		return fmt.Sprintf("%d B", v)
	}
	div, exp := int64(unit), 0
	for n := v / unit; n >= unit && exp < 5; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	return fmt.Sprintf("%.1f %s", float64(v)/float64(div), units[exp])
}

func formatRemaining(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	seconds := int(d.Round(time.Second).Seconds())
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	if h > 99 {
		return "--:--:--"
	}
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func currentRegistryIndexMustInRange() {
	if currentRegistryIndex < 0 || currentRegistryIndex >= len(currentRegistryList) {
		panic("current registry index is out of range")
	}
}

func notifyRegistryList(registries []model.Registry) {
	currentRegistryList = registries
	currentRegistryIndex = -1
	if package_manager.GetConfig().ShouldOutputJSON {
		out := make(map[string]interface{}, len(registries))
		for _, registry := range registries {
			out[registry.ID] = map[string]interface{}{
				"url":        registry.URL,
				"updated_at": time.Unix(registry.UpdatedAt/int64(time.Second), registry.UpdatedAt%int64(time.Second)).UTC().Format(time.RFC3339Nano),
			}
		}

		encoder := json.NewEncoder(os.Stdout)
		encoder.Encode(out)
	} else {
		if shouldUsePlainTextOutput() {
			notifyRenderer = &plainTextNotifier{}
		} else {
			notifyRenderer = newTUINotifier()
		}
		notifyRenderer.notifyRegistryList(registries)
	}
}

func notifyNextRegistry() {
	if currentRegistryIndex+1 >= len(currentRegistryList) {
		panic("no more registry to update")
	}
	currentRegistryIndex++
	registry := currentRegistryList[currentRegistryIndex]
	if package_manager.GetConfig().ShouldOutputJSON {
		out := map[string]interface{}{
			"event": "NEXT_REGISTRY",
			"index": currentRegistryIndex,
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.Encode(out)
	} else {
		notifyRenderer.notifyNextRegistry(currentRegistryIndex, registry)
	}
}

func notifyDownloadProgress(totalBytes int64, downloadedBytes int64, remaining time.Duration, speedBytesPerSecond float64) {
	currentRegistryIndexMustInRange()

	if package_manager.GetConfig().ShouldOutputJSON {
		out := map[string]interface{}{
			"event":                  "DOWNLOAD_PROGRESS",
			"total_bytes":            totalBytes,
			"downloaded_bytes":       downloadedBytes,
			"remaining_seconds":      int(remaining.Seconds()),
			"speed_bytes_per_second": speedBytesPerSecond,
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.Encode(out)
	} else {
		registry := currentRegistryList[currentRegistryIndex]
		notifyRenderer.notifyDownloadProgress(currentRegistryIndex, registry, totalBytes, downloadedBytes, remaining, speedBytesPerSecond)
	}
}

func percentFloat(totalBytes int64, downloadedBytes int64) float64 {
	if totalBytes <= 0 {
		return 0
	}
	p := float64(downloadedBytes) / float64(totalBytes)
	if p < 0 {
		return 0
	}
	if p > 1 {
		return 1
	}
	return p
}

func notifyDownloadError(message string) {
	currentRegistryIndexMustInRange()

	if package_manager.GetConfig().ShouldOutputJSON {
		out := map[string]interface{}{
			"event":   "DOWNLOAD_ERROR",
			"message": message,
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.Encode(out)
	} else {
		registry := currentRegistryList[currentRegistryIndex]
		notifyRenderer.notifyDownloadError(currentRegistryIndex, registry, message)
	}
}

func notifyValidationError(message string) {
	currentRegistryIndexMustInRange()

	if package_manager.GetConfig().ShouldOutputJSON {
		out := map[string]interface{}{
			"event":   "VALIDATION_ERROR",
			"message": message,
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.Encode(out)
	} else {
		registry := currentRegistryList[currentRegistryIndex]
		notifyRenderer.notifyValidationError(currentRegistryIndex, registry, message)
	}
}

func notifyRepositoryError(message string) {
	currentRegistryIndexMustInRange()

	if package_manager.GetConfig().ShouldOutputJSON {
		out := map[string]interface{}{
			"event":   "REPOSITORY_ERROR",
			"message": message,
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.Encode(out)
	} else {
		registry := currentRegistryList[currentRegistryIndex]
		notifyRenderer.notifyRepositoryError(currentRegistryIndex, registry, message)
	}
}

func notifyResult(removed int, added int) {
	currentRegistryIndexMustInRange()

	if package_manager.GetConfig().ShouldOutputJSON {
		out := map[string]interface{}{
			"event":   "RESULT",
			"removed": removed,
			"added":   added,
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.Encode(out)
	} else {
		registry := currentRegistryList[currentRegistryIndex]
		notifyRenderer.notifyResult(currentRegistryIndex, registry, removed, added)
	}
}
