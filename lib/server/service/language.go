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

package service

import (
	"diffscope-synthesis-platform/native"
	"sync"
)

var (
	languageJobQueueMu   sync.Mutex
	languageJobQueueCond = sync.NewCond(&languageJobQueueMu)
	languageJobQueue     []*LanguageJobContext
	languageJobQueueOnce sync.Once
)

type LanguageDataNote struct {
	Lyric                   string
	Language                string
	GraphemeType            string
	Pronunciation           string
	CandidatePronunciations []string
	IsNonTextOmittable      bool
	IsError                 bool
}

type LanguageData struct {
	Notes                []LanguageDataNote
	PronunciationTypeMap map[string]string
	PreferredLanguages   []string
	GraphemeTypePriority []string
}

type TaskType int

const (
	TaskTypeSplit TaskType = iota
	TaskTypeTag
	TaskTypeConvert
)

type LanguageJobContext struct {
	Data     LanguageData
	TaskList []TaskType
	Notifier func(TaskType, LanguageData) bool
}

func (c *LanguageJobContext) execute() {
	frontTask := c.TaskList[0]
	switch frontTask {
	case TaskTypeSplit:
		input := native.NewStringVector(int64(len(c.Data.Notes)))
		defer native.DeleteStringVector(input)
		for i, text := range c.Data.Notes {
			input.Set(i, text.Lyric)
		}
		splitTexts := native.LanguageServiceSplit_ReturnValueNeedsDeferDelete(input)
		defer native.DeleteStringVector(splitTexts)
		c.Data.Notes = make([]LanguageDataNote, splitTexts.Size())
		for i := int64(0); i < splitTexts.Size(); i++ {
			c.Data.Notes[i] = LanguageDataNote{
				Lyric: splitTexts.Get(int(i)),
			}
		}
	case TaskTypeTag:
		input := native.NewLanguageServiceTaggedNoteVector(int64(len(c.Data.Notes)))
		defer native.DeleteLanguageServiceTaggedNoteVector(input)
		for i, note := range c.Data.Notes {
			taggedNote := native.NewLanguageServiceTaggedNote()
			defer native.DeleteLanguageServiceTaggedNote(taggedNote)
			taggedNote.SetLyric(note.Lyric)
			input.Set(i, taggedNote)
		}
		preferredLanguages := native.NewStringVector(int64(len(c.Data.PreferredLanguages)))
		defer native.DeleteStringVector(preferredLanguages)
		for i, lang := range c.Data.PreferredLanguages {
			preferredLanguages.Set(i, lang)
		}
		graphemeTypePriority := native.NewStringVector(int64(len(c.Data.GraphemeTypePriority)))
		defer native.DeleteStringVector(graphemeTypePriority)
		for i, graphemeType := range c.Data.GraphemeTypePriority {
			graphemeTypePriority.Set(i, graphemeType)
		}
		native.LanguageServiceTagInPlace(input, preferredLanguages, graphemeTypePriority)
		for i := int64(0); i < input.Size(); i++ {
			taggedNote := input.Get(int(i))
			c.Data.Notes[i].Language = taggedNote.Language()
			c.Data.Notes[i].GraphemeType = taggedNote.GraphemeType()
			c.Data.Notes[i].IsNonTextOmittable = taggedNote.IsNonTextOmittable()
		}
	case TaskTypeConvert:
		input := native.NewLanguageServiceConvertedNoteVector(int64(len(c.Data.Notes)))
		defer native.DeleteLanguageServiceConvertedNoteVector(input)
		for i, note := range c.Data.Notes {
			convertedNote := native.NewLanguageServiceConvertedNote()
			defer native.DeleteLanguageServiceConvertedNote(convertedNote)
			convertedNote.SetLyric(note.Lyric)
			if pronunciationType, ok := c.Data.PronunciationTypeMap[note.Language]; ok {
				convertedNote.SetPronunciationType(pronunciationType)
			} else {
				convertedNote.SetPronunciationType(note.Language)
			}
			input.Set(i, convertedNote)
		}
		native.LanguageServiceConvertInPlace(input)
		for i := int64(0); i < input.Size(); i++ {
			convertedNote := input.Get(int(i))
			c.Data.Notes[i].Pronunciation = convertedNote.Pronunciation()
			c.Data.Notes[i].CandidatePronunciations = make([]string, convertedNote.CandidatePronunciations().Size())
			for j := int64(0); j < convertedNote.CandidatePronunciations().Size(); j++ {
				c.Data.Notes[i].CandidatePronunciations[j] = convertedNote.CandidatePronunciations().Get(int(j))
			}
			c.Data.Notes[i].IsError = convertedNote.IsError()
		}
	}
}

func (c *LanguageJobContext) notifyAndStepForward() *LanguageJobContext {
	frontTask := c.TaskList[0]
	if !c.Notifier(frontTask, c.Data) {
		return nil
	}
	c.TaskList = c.TaskList[1:]
	if len(c.TaskList) == 0 {
		return nil
	}
	return c
}

func startLanguageJobWorker() {
	for {
		languageJobQueueMu.Lock()
		for len(languageJobQueue) == 0 {
			languageJobQueueCond.Wait()
		}
		ctx := languageJobQueue[0]
		languageJobQueue = languageJobQueue[1:]
		languageJobQueueMu.Unlock()

		ctx.execute()
		if next := ctx.notifyAndStepForward(); next != nil {
			enqueueLanguageJob(next)
		}
	}
}

func enqueueLanguageJob(ctx *LanguageJobContext) {
	languageJobQueueMu.Lock()
	languageJobQueue = append(languageJobQueue, ctx)
	languageJobQueueMu.Unlock()
	languageJobQueueCond.Signal()
}

func SubmitLanguageJob(ctx *LanguageJobContext) {
	if ctx == nil || len(ctx.TaskList) == 0 {
		return
	}
	languageJobQueueOnce.Do(func() {
		go startLanguageJobWorker()
	})
	enqueueLanguageJob(ctx)
}
