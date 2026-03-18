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

package controller

import (
	"diffscope-synthesis-platform/internal/service"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type languageRequestNote struct {
	Lyric string `json:"lyric"`
}

type languageRequestConfig struct {
	Stream               *bool    `json:"stream"`
	PreferredLanguages   []string `json:"preferred_languages"`
	GraphemeTypePriority []string `json:"grapheme_type_priority"`
}

type languageRequest struct {
	Notes  *[]languageRequestNote `json:"notes"`
	Task   *[]string              `json:"task"`
	Config *languageRequestConfig `json:"config"`
}

type languageResponseNote struct {
	Lyric                   *string  `json:"lyric,omitempty"`
	Language                *string  `json:"language,omitempty"`
	GraphemeType            *string  `json:"grapheme_type,omitempty"`
	Pronunciation           *string  `json:"pronunciation,omitempty"`
	CandidatePronunciations []string `json:"candidate_pronunciations,omitempty"`
	NonTextOmittable        *bool    `json:"non_text_omittable,omitempty"`
	Error                   *bool    `json:"error,omitempty"`
}

type languageResponse struct {
	Status    languageStatus         `json:"status"`
	Timestamp string                 `json:"timestamp"`
	Notes     []languageResponseNote `json:"notes,omitempty"`
	Task      []languageTaskName     `json:"task"`
}

type languageTaskDef struct {
	Name languageTaskName
	Type service.TaskType
}

var orderedLanguageTaskDefs = []languageTaskDef{
	{Name: languageTaskSplit, Type: service.TaskTypeSplit},
	{Name: languageTaskTag, Type: service.TaskTypeTag},
	{Name: languageTaskConvert, Type: service.TaskTypeConvert},
}

func postLanguage(c *gin.Context) {
	var req languageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Notes == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "notes is required"})
		return
	}
	if req.Task == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task is required"})
		return
	}

	notes := *req.Notes
	rawTasks := *req.Task

	streamEnabled := false
	preferredLanguages := []string{}
	graphemeTypePriority := []string{}
	if req.Config != nil {
		if req.Config.Stream != nil {
			streamEnabled = *req.Config.Stream
		}
		preferredLanguages = append(preferredLanguages, req.Config.PreferredLanguages...)
		graphemeTypePriority = append(graphemeTypePriority, req.Config.GraphemeTypePriority...)
	}

	taskNames, taskTypes, err := normalizeLanguageTasks(rawTasks)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	jobData := service.LanguageData{
		Notes:                make([]service.LanguageDataNote, len(notes)),
		PreferredLanguages:   preferredLanguages,
		GraphemeTypePriority: graphemeTypePriority,
	}
	for i, note := range notes {
		jobData.Notes[i] = service.LanguageDataNote{Lyric: note.Lyric}
	}

	if len(taskTypes) == 0 {
		resp := languageResponse{
			Status:    languageStatusFinished,
			Timestamp: utcTimestamp(),
			Task:      []languageTaskName{},
		}
		if streamEnabled {
			setupStreamResponse(c)
			_ = writeNDJSON(c, resp)
			return
		}
		c.JSON(http.StatusOK, resp)
		return
	}

	if streamEnabled {
		handleLanguageStream(c, jobData, taskTypes)
		return
	}

	handleLanguageNonStream(c, jobData, taskNames, taskTypes)
}

func normalizeLanguageTasks(rawTasks []string) ([]languageTaskName, []service.TaskType, error) {
	selected := make(map[languageTaskName]bool, len(orderedLanguageTaskDefs))
	for _, rawTask := range rawTasks {
		taskName, ok := parseLanguageTaskName(rawTask)
		if !ok {
			normalized := strings.ToUpper(strings.TrimSpace(rawTask))
			if normalized == "" {
				continue
			}
			return nil, nil, errBadLanguageTask(normalized)
		}
		if taskName == "" {
			continue
		}
		selected[taskName] = true
	}

	if selected[languageTaskConvert] && !selected[languageTaskTag] {
		return nil, nil, errBadLanguageTaskCombination("CONVERT requires TAG")
	}

	orderedTaskNames := make([]languageTaskName, 0, len(selected))
	orderedTaskTypes := make([]service.TaskType, 0, len(selected))
	for _, taskDef := range orderedLanguageTaskDefs {
		if selected[taskDef.Name] {
			orderedTaskNames = append(orderedTaskNames, taskDef.Name)
			orderedTaskTypes = append(orderedTaskTypes, taskDef.Type)
		}
	}
	return orderedTaskNames, orderedTaskTypes, nil
}

func handleLanguageStream(c *gin.Context, data service.LanguageData, taskTypes []service.TaskType) {
	setupStreamResponse(c)
	if !writeNDJSON(c, languageResponse{
		Status:    languageStatusPartial,
		Timestamp: utcTimestamp(),
		Task:      []languageTaskName{},
	}) {
		return
	}

	taskEvents := make(chan languageTaskEvent, len(taskTypes))

	notifier := func(task service.TaskType, currentData service.LanguageData) bool {
		select {
		case <-c.Request.Context().Done():
			return false
		default:
		}

		event := languageTaskEvent{
			TaskName: taskNameFromType(task),
			Data:     cloneLanguageData(currentData),
		}

		select {
		case taskEvents <- event:
			return true
		case <-c.Request.Context().Done():
			return false
		}
	}

	service.SubmitLanguageJob(&service.LanguageJobContext{
		Data:     data,
		TaskList: taskTypes,
		Notifier: notifier,
	})

	for i := 0; i < len(taskTypes); i++ {
		select {
		case <-c.Request.Context().Done():
			return
		case event := <-taskEvents:
			status := languageStatusPartial
			if i == len(taskTypes)-1 {
				status = languageStatusFinished
			}
			resp := languageResponse{
				Status:    status,
				Timestamp: utcTimestamp(),
				Notes:     buildTaskResponseNotes(event.Data, event.TaskName),
				Task:      []languageTaskName{event.TaskName},
			}
			if !writeNDJSON(c, resp) {
				return
			}
		}
	}
}

func handleLanguageNonStream(c *gin.Context, data service.LanguageData, taskNames []languageTaskName, taskTypes []service.TaskType) {
	resultCh := make(chan service.LanguageData, 1)
	finalTask := taskTypes[len(taskTypes)-1]

	notifier := func(task service.TaskType, currentData service.LanguageData) bool {
		select {
		case <-c.Request.Context().Done():
			return false
		default:
		}

		if task == finalTask {
			select {
			case resultCh <- cloneLanguageData(currentData):
			default:
			}
		}
		return true
	}

	service.SubmitLanguageJob(&service.LanguageJobContext{
		Data:     data,
		TaskList: taskTypes,
		Notifier: notifier,
	})

	select {
	case <-c.Request.Context().Done():
		return
	case finalData := <-resultCh:
		selected := make(map[languageTaskName]bool, len(taskNames))
		for _, taskName := range taskNames {
			selected[taskName] = true
		}
		resp := languageResponse{
			Status:    languageStatusFinished,
			Timestamp: utcTimestamp(),
			Notes:     buildMergedResponseNotes(finalData, selected),
			Task:      append([]languageTaskName(nil), taskNames...),
		}
		c.JSON(http.StatusOK, resp)
	}
}

func setupStreamResponse(c *gin.Context) {
	headers := c.Writer.Header()
	headers.Set("Content-Type", "application/x-ndjson; charset=utf-8")
	headers.Set("Cache-Control", "no-cache")
	c.Status(http.StatusOK)
}

func writeNDJSON(c *gin.Context, resp languageResponse) bool {
	encoded, err := json.Marshal(resp)
	if err != nil {
		return false
	}
	if _, err := c.Writer.Write(append(encoded, '\n')); err != nil {
		return false
	}
	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}
	return true
}

func taskNameFromType(task service.TaskType) languageTaskName {
	for _, taskDef := range orderedLanguageTaskDefs {
		if taskDef.Type == task {
			return taskDef.Name
		}
	}
	return ""
}

func buildTaskResponseNotes(data service.LanguageData, taskName languageTaskName) []languageResponseNote {
	notes := make([]languageResponseNote, len(data.Notes))
	for i, note := range data.Notes {
		respNote := languageResponseNote{}
		switch taskName {
		case languageTaskSplit:
			lyric := note.Lyric
			respNote.Lyric = &lyric
		case languageTaskTag:
			language := note.Language
			graphemeType := note.GraphemeType
			nonTextOmittable := note.IsNonTextOmittable
			respNote.Language = &language
			respNote.GraphemeType = &graphemeType
			respNote.NonTextOmittable = &nonTextOmittable
		case languageTaskConvert:
			pronunciation := note.Pronunciation
			errorFlag := note.IsError
			respNote.Pronunciation = &pronunciation
			respNote.CandidatePronunciations = append([]string(nil), note.CandidatePronunciations...)
			respNote.Error = &errorFlag
		}
		notes[i] = respNote
	}
	return notes
}

func buildMergedResponseNotes(data service.LanguageData, selectedTasks map[languageTaskName]bool) []languageResponseNote {
	notes := make([]languageResponseNote, len(data.Notes))
	for i, note := range data.Notes {
		respNote := languageResponseNote{}
		if selectedTasks[languageTaskSplit] {
			lyric := note.Lyric
			respNote.Lyric = &lyric
		}
		if selectedTasks[languageTaskTag] {
			language := note.Language
			graphemeType := note.GraphemeType
			nonTextOmittable := note.IsNonTextOmittable
			respNote.Language = &language
			respNote.GraphemeType = &graphemeType
			respNote.NonTextOmittable = &nonTextOmittable
		}
		if selectedTasks[languageTaskConvert] {
			pronunciation := note.Pronunciation
			errorFlag := note.IsError
			respNote.Pronunciation = &pronunciation
			respNote.CandidatePronunciations = append([]string(nil), note.CandidatePronunciations...)
			respNote.Error = &errorFlag
		}
		notes[i] = respNote
	}
	return notes
}

func cloneLanguageData(data service.LanguageData) service.LanguageData {
	copied := service.LanguageData{
		Notes:                make([]service.LanguageDataNote, len(data.Notes)),
		PreferredLanguages:   append([]string(nil), data.PreferredLanguages...),
		GraphemeTypePriority: append([]string(nil), data.GraphemeTypePriority...),
	}
	for i, note := range data.Notes {
		copied.Notes[i] = note
		copied.Notes[i].CandidatePronunciations = append([]string(nil), note.CandidatePronunciations...)
	}
	return copied
}

func utcTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

type languageTaskEvent struct {
	TaskName languageTaskName
	Data     service.LanguageData
}

type languageStatus string

const (
	languageStatusPartial  languageStatus = "PARTIAL"
	languageStatusFinished languageStatus = "FINISHED"
)

type languageTaskName string

const (
	languageTaskSplit   languageTaskName = "SPLIT"
	languageTaskTag     languageTaskName = "TAG"
	languageTaskConvert languageTaskName = "CONVERT"
)

var languageTaskNameByString = map[string]languageTaskName{
	string(languageTaskSplit):   languageTaskSplit,
	string(languageTaskTag):     languageTaskTag,
	string(languageTaskConvert): languageTaskConvert,
}

func parseLanguageTaskName(raw string) (languageTaskName, bool) {
	normalized := strings.ToUpper(strings.TrimSpace(raw))
	if normalized == "" {
		return "", true
	}
	taskName, ok := languageTaskNameByString[normalized]
	return taskName, ok
}

type languageTaskError string

func (e languageTaskError) Error() string {
	return string(e)
}

func errBadLanguageTask(taskName string) error {
	return languageTaskError("unsupported task: " + taskName)
}

func errBadLanguageTaskCombination(reason string) error {
	return languageTaskError("invalid task combination: " + reason)
}
