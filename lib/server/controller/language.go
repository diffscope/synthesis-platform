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
	"diffscope-synthesis-platform/lib/server/service"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/creasty/defaults"
	"github.com/gin-gonic/gin"
)

type languageRequestNote struct {
	// Required for SPLIT, TAG, and CONVERT tasks
	Lyric *string `json:"lyric"`

	// Required and non-empty for CONVERT task
	Language *string `json:"language"`
}

type languageRequestConfig struct {
	// Optional. If set to true, the server will send intermediate results after each task as NDJSON stream.
	Stream bool `json:"stream" default:"true"`

	// Optional. Map of language to pronunciation type.
	PronunciationTypeMap map[string]string `json:"pronunciation_type_map"`

	// Optional. List of preferred languages for language tagging and pronunciation conversion. Higher priority languages should be placed earlier in the list.
	PreferredLanguages []string `json:"preferred_languages"`

	// Optional. // TODO
	GraphemeTypePriority []string `json:"grapheme_type_priority"`
}

type languageRequest struct {
	Notes  []languageRequestNote `json:"notes" binding:"required"`
	Task   []languageTaskName    `json:"task" binding:"required"`
	Config languageRequestConfig `json:"config"`
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

func parseLanguageRequest(c *gin.Context) *languageRequest {
	var req languageRequest
	defaults.Set(&req)
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return nil
	}

	normalizedTasks, err := normalizeLanguageTasks(req.Task)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return nil
	}

	for _, note := range req.Notes {
		if len(normalizedTasks) > 0 && note.Lyric == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "notes[].lyric is required for SPLIT, TAG, or CONVERT tasks"})
			return nil
		}
		if isConvertOnlyTaskSet(normalizedTasks) && note.Language == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "notes[].language is required when task contains only CONVERT"})
			return nil
		}
	}
	return &req
}

func PostLanguage(c *gin.Context) {
	req := parseLanguageRequest(c)
	if req == nil {
		return
	}

	jobData := service.LanguageData{
		Notes:                make([]service.LanguageDataNote, len(req.Notes)),
		PronunciationTypeMap: req.Config.PronunciationTypeMap,
		PreferredLanguages:   req.Config.PreferredLanguages,
		GraphemeTypePriority: req.Config.GraphemeTypePriority,
	}
	for i, note := range req.Notes {
		jobData.Notes[i] = service.LanguageDataNote{
			Lyric:    optionalStringValue(note.Lyric),
			Language: optionalStringValue(note.Language),
		}
	}

	if len(req.Task) == 0 {
		resp := languageResponse{
			Status:    languageStatusFinished,
			Timestamp: utcTimestamp(),
			Task:      []languageTaskName{},
		}
		if req.Config.Stream {
			setupStreamResponse(c)
			writeNDJSON(c, resp)
			return
		}
		c.JSON(http.StatusOK, resp)
		return
	}

	taskTypes := make([]service.TaskType, len(req.Task))
	for i, taskName := range req.Task {
		taskTypes[i] = taskNameToType(taskName)
	}
	sort.Slice(taskTypes, func(i, j int) bool {
		return taskTypes[i] < taskTypes[j]
	})

	if req.Config.Stream {
		handleLanguageStream(c, jobData, taskTypes)
		return
	}

	handleLanguageNonStream(c, jobData, taskTypes)
}

func normalizeLanguageTasks(rawTasks []languageTaskName) ([]languageTaskName, error) {
	selected := make(map[languageTaskName]bool, 3)
	for _, rawTask := range rawTasks {
		selected[rawTask] = true
	}
	if selected[languageTaskSplit] && selected[languageTaskConvert] && !selected[languageTaskTag] {
		return nil, errBadLanguageTaskCombination("SPLIT + CONVERT requires TAG")
	}

	taskNames := make([]languageTaskName, 0, len(selected))
	for taskName := range selected {
		taskNames = append(taskNames, taskName)
	}
	return taskNames, nil
}

func handleLanguageStream(c *gin.Context, data service.LanguageData, taskTypes []service.TaskType) {
	setupStreamResponse(c)
	writeNDJSON(c, languageResponse{
		Status:    languageStatusPartial,
		Timestamp: utcTimestamp(),
		Task:      []languageTaskName{},
	})

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
			writeNDJSON(c, resp)
		}
	}
}

func handleLanguageNonStream(c *gin.Context, data service.LanguageData, taskTypes []service.TaskType) {
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
		selected := make(map[languageTaskName]bool, 3)
		taskNames := make([]languageTaskName, len(taskTypes))
		for i, taskType := range taskTypes {
			taskName := taskNameFromType(taskType)
			selected[taskName] = true
			taskNames[i] = taskName
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

func writeNDJSON(c *gin.Context, resp languageResponse) {
	encoded, err := json.Marshal(resp)
	if err != nil {
		panic(err.Error())
	}
	if _, err := c.Writer.Write(append(encoded, '\n')); err != nil {
		return
	}
	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}
}

func taskNameFromType(task service.TaskType) languageTaskName {
	switch task {
	case service.TaskTypeSplit:
		return languageTaskSplit
	case service.TaskTypeTag:
		return languageTaskTag
	case service.TaskTypeConvert:
		return languageTaskConvert
	default:
		panic(fmt.Sprintf("unknown task type: %d", task))
	}
}

func taskNameToType(name languageTaskName) service.TaskType {
	switch name {
	case languageTaskSplit:
		return service.TaskTypeSplit
	case languageTaskTag:
		return service.TaskTypeTag
	case languageTaskConvert:
		return service.TaskTypeConvert
	default:
		panic(fmt.Sprintf("unknown task name: %s", name))
	}
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
		PronunciationTypeMap: make(map[string]string, len(data.PronunciationTypeMap)),
		PreferredLanguages:   append([]string(nil), data.PreferredLanguages...),
		GraphemeTypePriority: append([]string(nil), data.GraphemeTypePriority...),
	}
	for language, pronunciationType := range data.PronunciationTypeMap {
		copied.PronunciationTypeMap[language] = pronunciationType
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

func isConvertOnlyTaskSet(taskNames []languageTaskName) bool {
	return len(taskNames) == 1 && taskNames[0] == languageTaskConvert
}

func optionalStringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

type languageTaskError string

func (e languageTaskError) Error() string {
	return string(e)
}

func errBadLanguageTaskCombination(reason string) error {
	return languageTaskError("invalid task combination: " + reason)
}
