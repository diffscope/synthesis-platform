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

package diffsinger

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"diffscope-synthesis-platform/internal/api"
	"diffscope-synthesis-platform/internal/phonemeconversion"
	"diffscope-synthesis-platform/internal/utils"
)

const (
	slurPronunciation = "-"
	plusPronunciation = "+"
)

type singerLanguageKey struct {
	Singer   SingerIdentifier
	Language string
}

type phonemeWorkItem struct {
	index         int
	pronunciation string
}

type phonemeLanguageGroup struct {
	language SingerLanguage
	items    []phonemeWorkItem
}

type leasedS2P struct {
	value     *phonemeconversion.S2P
	custom    bool
	trimInput bool
	release   func()
}

type leasedOnsetMarker struct {
	value   *phonemeconversion.OnsetMarker
	custom  bool
	release func()
}

var (
	mapS2PMu       sync.Mutex
	mapS2PResource = utils.NewResourceManager[string, *phonemeconversion.S2P](
		0,
		0,
		func(_ string, value *phonemeconversion.S2P) {
			value.Close()
		},
	)

	dictS2PMu       sync.Mutex
	dictS2PResource = utils.NewResourceManager[string, *phonemeconversion.S2P](
		0,
		0,
		func(_ string, value *phonemeconversion.S2P) {
			value.Close()
		},
	)

	customS2PMu       sync.Mutex
	customS2PResource = utils.NewResourceManager[singerLanguageKey, *phonemeconversion.S2P](
		0,
		0,
		func(_ singerLanguageKey, value *phonemeconversion.S2P) {
			value.Close()
		},
	)

	ruleOnsetMu       sync.Mutex
	ruleOnsetResource = utils.NewResourceManager[string, *phonemeconversion.OnsetMarker](
		0,
		0,
		func(_ string, value *phonemeconversion.OnsetMarker) {
			value.Close()
		},
	)

	customOnsetMu       sync.Mutex
	customOnsetResource = utils.NewResourceManager[singerLanguageKey, *phonemeconversion.OnsetMarker](
		0,
		0,
		func(_ singerLanguageKey, value *phonemeconversion.OnsetMarker) {
			value.Close()
		},
	)

	phonemeHashMu   sync.Mutex
	s2pFileHashes   = make(map[singerLanguageKey]string)
	onsetFileHashes = make(map[singerLanguageKey]string)
)

func (Architecture) Phoneme(
	ctx context.Context,
	archExtra json.RawMessage,
	singer api.Singer,
	notes []api.PronunciationNote,
) ([]api.PhonemeNote, error) {
	extra, err := parseArchExtra(archExtra)
	if err != nil {
		return nil, err
	}
	_ = extra

	singerID, ok := getSingerIdentifier(singer)
	if !ok {
		return nil, api.NewSingerNotExistError(singer.ID, "")
	}
	metadata, ok := GetSinger(singerID)
	if !ok {
		return nil, api.NewSingerNotExistError(singer.ID, "")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	result := makeEmptyPhonemeNotes(len(notes))
	groups := groupPhonemeNotes(singerID, metadata, notes)
	phonemeBuffers := make([]*phonemeconversion.PhonemeBuffer, len(notes))
	defer closePhonemeBuffers(phonemeBuffers)

	for key, group := range groups {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := convertPhonemeGroup(ctx, key, group, phonemeBuffers); err != nil {
			return nil, err
		}
	}

	for index, buffer := range phonemeBuffers {
		if buffer != nil {
			result[index].Phonemes = buffer.Snapshot()
		}
	}
	postProcessPlusPronunciations(notes, result)
	return result, nil
}

func configurePhonemeResourceManagers() {
	configurePhonemeResourceManager(mapS2PResource)
	configurePhonemeResourceManager(dictS2PResource)
	configurePhonemeResourceManager(customS2PResource)
	configurePhonemeResourceManager(ruleOnsetResource)
	configurePhonemeResourceManager(customOnsetResource)
}

func configurePhonemeResourceManager[K comparable, V any](manager *utils.ResourceManager[K, V]) {
	manager.SetTimeout(getPhonemeCleanupTimeout())
	manager.SetScanInterval(getPhonemeCleanupInterval())
}

func makeEmptyPhonemeNotes(count int) []api.PhonemeNote {
	result := make([]api.PhonemeNote, count)
	for index := range result {
		result[index] = api.PhonemeNote{
			Phonemes: []api.Phoneme{},
		}
	}
	return result
}

func groupPhonemeNotes(
	singerID SingerIdentifier,
	metadata SingerMetadata,
	notes []api.PronunciationNote,
) map[singerLanguageKey]phonemeLanguageGroup {
	groups := make(map[singerLanguageKey]phonemeLanguageGroup)
	for index, note := range notes {
		if note.Pronunciation == slurPronunciation || note.Pronunciation == plusPronunciation {
			continue
		}

		language, ok := metadata.Languages[note.Language]
		if !ok {
			continue
		}

		key := singerLanguageKey{
			Singer:   singerID,
			Language: note.Language,
		}
		group := groups[key]
		group.language = language
		group.items = append(group.items, phonemeWorkItem{
			index:         index,
			pronunciation: note.Pronunciation,
		})
		groups[key] = group
	}
	return groups
}

func convertPhonemeGroup(
	ctx context.Context,
	key singerLanguageKey,
	group phonemeLanguageGroup,
	phonemeBuffers []*phonemeconversion.PhonemeBuffer,
) error {
	s2p, err := acquireS2P(key, group.language)
	if err != nil {
		return err
	}
	defer s2p.release()

	if err := runS2P(ctx, group.items, s2p, phonemeBuffers); err != nil {
		return err
	}

	onsetMarker, err := acquireOnsetMarker(key, group.language)
	if err != nil {
		return err
	}
	defer onsetMarker.release()

	return runOnsetMarker(ctx, group.items, onsetMarker, phonemeBuffers)
}

func acquireS2P(key singerLanguageKey, language SingerLanguage) (leasedS2P, error) {
	switch language.S2PMode {
	case S2PModeDirect:
		s2p, err := phonemeconversion.NewDirectS2P()
		if err != nil {
			return leasedS2P{}, singerConfigError("create direct S2P", err)
		}
		return leasedS2P{
			value:     s2p,
			trimInput: true,
			release: func() {
				s2p.Close()
			},
		}, nil
	case S2PModeMap:
		hash, err := hashFile(language.S2PFile)
		if err != nil {
			return leasedS2P{}, singerConfigError("hash S2P file", err)
		}
		recordS2PFileHash(key, hash)
		lease, err := acquireResource(&mapS2PMu, mapS2PResource, hash, func() (*phonemeconversion.S2P, error) {
			return phonemeconversion.NewMapS2P(language.S2PFile)
		})
		if err != nil {
			return leasedS2P{}, singerConfigError("create map S2P", err)
		}
		return leasedS2P{
			value:     lease.Value(),
			trimInput: true,
			release: func() {
				lease.Release()
			},
		}, nil
	case S2PModeDict:
		hash, err := hashFile(language.S2PFile)
		if err != nil {
			return leasedS2P{}, singerConfigError("hash S2P file", err)
		}
		recordS2PFileHash(key, hash)
		lease, err := acquireResource(&dictS2PMu, dictS2PResource, hash, func() (*phonemeconversion.S2P, error) {
			return phonemeconversion.NewDictS2P(language.S2PFile)
		})
		if err != nil {
			return leasedS2P{}, singerConfigError("create dict S2P", err)
		}
		return leasedS2P{
			value: lease.Value(),
			release: func() {
				lease.Release()
			},
		}, nil
	case S2PModeCustom:
		lease, err := acquireResource(&customS2PMu, customS2PResource, key, func() (*phonemeconversion.S2P, error) {
			return phonemeconversion.NewCustomS2P(language.S2PFile)
		})
		if err != nil {
			return leasedS2P{}, singerConfigError("create custom S2P", err)
		}
		return leasedS2P{
			value:  lease.Value(),
			custom: true,
			release: func() {
				lease.Release()
			},
		}, nil
	default:
		return leasedS2P{}, singerConfigError("create S2P", fmt.Errorf("unsupported S2P mode %q", language.S2PMode))
	}
}

func acquireOnsetMarker(key singerLanguageKey, language SingerLanguage) (leasedOnsetMarker, error) {
	switch language.OnsetMode {
	case OnsetModeRule:
		hash, err := hashFile(language.OnsetFile)
		if err != nil {
			return leasedOnsetMarker{}, singerConfigError("hash onset file", err)
		}
		recordOnsetFileHash(key, hash)
		lease, err := acquireResource(&ruleOnsetMu, ruleOnsetResource, hash, func() (*phonemeconversion.OnsetMarker, error) {
			return phonemeconversion.NewRuleOnsetMarker(language.OnsetFile)
		})
		if err != nil {
			return leasedOnsetMarker{}, singerConfigError("create rule onset marker", err)
		}
		return leasedOnsetMarker{
			value: lease.Value(),
			release: func() {
				lease.Release()
			},
		}, nil
	case OnsetModeCustom:
		lease, err := acquireResource(&customOnsetMu, customOnsetResource, key, func() (*phonemeconversion.OnsetMarker, error) {
			return phonemeconversion.NewCustomOnsetMarker(language.OnsetFile)
		})
		if err != nil {
			return leasedOnsetMarker{}, singerConfigError("create custom onset marker", err)
		}
		return leasedOnsetMarker{
			value:  lease.Value(),
			custom: true,
			release: func() {
				lease.Release()
			},
		}, nil
	default:
		return leasedOnsetMarker{}, singerConfigError("create onset marker", fmt.Errorf("unsupported onset mode %q", language.OnsetMode))
	}
}

func acquireResource[K comparable, V any](
	mu *sync.Mutex,
	manager *utils.ResourceManager[K, V],
	key K,
	create func() (V, error),
) (*utils.ResourceLease[K, V], error) {
	mu.Lock()
	defer mu.Unlock()

	if lease, ok := manager.Acquire(key); ok {
		return lease, nil
	}

	value, err := create()
	if err != nil {
		return nil, err
	}
	manager.Put(key, value)

	lease, ok := manager.Acquire(key)
	if !ok {
		return nil, fmt.Errorf("acquire resource")
	}
	return lease, nil
}

func runS2P(
	ctx context.Context,
	items []phonemeWorkItem,
	s2p leasedS2P,
	phonemeBuffers []*phonemeconversion.PhonemeBuffer,
) error {
	return runParallelPhonemeOperation(ctx, s2p.custom, s2p.value.TerminateCustom, func(setError func(error)) {
		var wg sync.WaitGroup
		for _, item := range items {
			item := item
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := ctx.Err(); err != nil {
					setError(err)
					return
				}
				pronunciation := item.pronunciation
				if s2p.trimInput {
					pronunciation = strings.TrimSpace(pronunciation)
				}
				buffer, err := s2p.value.Convert(pronunciation)
				if err != nil {
					setError(err)
					return
				}
				phonemeBuffers[item.index] = buffer
			}()
		}
		wg.Wait()
	})
}

func runOnsetMarker(
	ctx context.Context,
	items []phonemeWorkItem,
	onsetMarker leasedOnsetMarker,
	phonemeBuffers []*phonemeconversion.PhonemeBuffer,
) error {
	return runParallelPhonemeOperation(ctx, onsetMarker.custom, onsetMarker.value.TerminateCustom, func(setError func(error)) {
		var wg sync.WaitGroup
		for _, item := range items {
			item := item
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := ctx.Err(); err != nil {
					setError(err)
					return
				}
				buffer := phonemeBuffers[item.index]
				if buffer == nil {
					return
				}
				if err := onsetMarker.value.Mark(buffer); err != nil {
					setError(err)
				}
			}()
		}
		wg.Wait()
	})
}

func runParallelPhonemeOperation(
	ctx context.Context,
	custom bool,
	terminate func(),
	run func(setError func(error)),
) error {
	var errMu sync.Mutex
	var firstErr error
	setError := func(err error) {
		if err == nil {
			return
		}
		errMu.Lock()
		defer errMu.Unlock()
		if firstErr == nil {
			firstErr = err
		}
	}
	getError := func() error {
		errMu.Lock()
		defer errMu.Unlock()
		return firstErr
	}

	done := make(chan struct{})
	go func() {
		run(setError)
		close(done)
	}()

	if !custom {
		select {
		case <-done:
			return phonemeRuntimeError(getError())
		case <-ctx.Done():
			<-done
			return ctx.Err()
		}
	}

	timeout := getPhonemeCustomWorkerTimeout()
	if timeout <= 0 {
		select {
		case <-done:
			return phonemeRuntimeError(getError())
		case <-ctx.Done():
			terminate()
			<-done
			return ctx.Err()
		}
	}

	timer := time.NewTimer(timeout)
	select {
	case <-done:
		timer.Stop()
		return phonemeRuntimeError(getError())
	case <-ctx.Done():
		timer.Stop()
		terminate()
		<-done
		return ctx.Err()
	case <-timer.C:
		terminate()
		<-done
		return api.NewError(api.ProblemTypeInternalError, "phoneme conversion timed out")
	}
}

func postProcessPlusPronunciations(notes []api.PronunciationNote, result []api.PhonemeNote) {
	for index, note := range notes {
		if note.Pronunciation != plusPronunciation || index == 0 {
			continue
		}

		previous := result[index-1].Phonemes
		splitIndex := secondOnsetIndex(previous)
		if splitIndex < 0 {
			continue
		}

		result[index].Phonemes = append([]api.Phoneme(nil), previous[splitIndex:]...)
		result[index-1].Phonemes = previous[:splitIndex]
	}
}

func secondOnsetIndex(phonemes []api.Phoneme) int {
	onsetCount := 0
	for index, phoneme := range phonemes {
		if !phoneme.Onset {
			continue
		}
		onsetCount++
		if onsetCount == 2 {
			return index
		}
	}
	return -1
}

func closePhonemeBuffers(buffers []*phonemeconversion.PhonemeBuffer) {
	for _, buffer := range buffers {
		buffer.Close()
	}
}

func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := sha512.Sum512(data)
	return hex.EncodeToString(hash[:]), nil
}

func recordS2PFileHash(key singerLanguageKey, hash string) {
	phonemeHashMu.Lock()
	defer phonemeHashMu.Unlock()
	s2pFileHashes[key] = hash
}

func recordOnsetFileHash(key singerLanguageKey, hash string) {
	phonemeHashMu.Lock()
	defer phonemeHashMu.Unlock()
	onsetFileHashes[key] = hash
}

func singerConfigError(action string, err error) error {
	detail := action
	if err == nil {
		return api.NewSingerConfigInvalidError(detail, api.ValidationIssue{
			Pointer: "#/context/singer",
			Type:    "configuration_invalid",
			Detail:  detail,
		})
	}
	detail = fmt.Sprintf("%s: %v", action, err)
	return api.NewSingerConfigInvalidError(detail, api.ValidationIssue{
		Pointer: "#/context/singer",
		Type:    "configuration_invalid",
		Detail:  detail,
	})
}

func phonemeRuntimeError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(*api.Error); ok {
		return err
	}
	return api.NewError(api.ProblemTypeInternalError, err.Error())
}
