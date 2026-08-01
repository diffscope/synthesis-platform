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
	"encoding/json"

	"diffscope-synthesis-platform/internal/api"
	"diffscope-synthesis-platform/internal/languageconversion"

	"github.com/diffscope/diffscope-package-manager/packageinfo"
)

const unknownG2PIdentifier = "g2p-unknown-official"

type pronunciationRequest struct {
	ctx    context.Context
	lyrics []languageconversion.Lyric
	result chan pronunciationResult
}

type pronunciationResult struct {
	pronunciations []api.Pronunciation
	err            error
}

var pronunciationRequests = make(chan pronunciationRequest)

func init() {
	go runPronunciationWorker()
}

func (Architecture) Pronunciation(
	ctx context.Context,
	archExtra json.RawMessage,
	singer api.Singer,
	lyrics []api.Lyric,
) ([]api.Pronunciation, error) {
	extra, err := parseArchExtra(archExtra)
	if err != nil {
		return nil, err
	}
	_ = extra

	metadata, ok := getSingerMetadata(singer)
	if !ok {
		return nil, api.NewSingerNotExistError(singer.ID, "")
	}

	input := make([]languageconversion.Lyric, 0, len(lyrics))
	for _, lyric := range lyrics {
		input = append(input, languageconversion.Lyric{
			Text:     lyric.Lyric,
			Language: getLyricG2PIdentifier(metadata, lyric.Language),
		})
	}

	request := pronunciationRequest{
		ctx:    ctx,
		lyrics: input,
		result: make(chan pronunciationResult, 1),
	}
	select {
	case pronunciationRequests <- request:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	select {
	case result := <-request.result:
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return result.pronunciations, result.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func runPronunciationWorker() {
	for request := range pronunciationRequests {
		if err := request.ctx.Err(); err != nil {
			request.result <- pronunciationResult{err: err}
			continue
		}
		pronunciations := languageconversion.Convert(request.lyrics)
		request.result <- pronunciationResult{
			pronunciations: convertPronunciations(pronunciations),
		}
	}
}

func getSingerMetadata(singer api.Singer) (SingerMetadata, bool) {
	id, ok := getSingerIdentifier(singer)
	if !ok {
		return SingerMetadata{}, false
	}
	return GetSinger(id)
}

func getSingerIdentifier(singer api.Singer) (SingerIdentifier, bool) {
	reference, err := packageinfo.ParsePackageReference(singer.ID)
	if err != nil {
		return SingerIdentifier{}, false
	}
	if reference.Type != packageinfo.PackageReferenceTypeSinger || reference.Version == nil {
		return SingerIdentifier{}, false
	}

	return SingerIdentifier{
		PackageID: reference.PackageID,
		Version:   *reference.Version,
		SingerID:  reference.SingerID,
	}, true
}

func getLyricG2PIdentifier(metadata SingerMetadata, language string) string {
	item, ok := metadata.Languages[language]
	if !ok {
		return unknownG2PIdentifier
	}
	return item.G2P
}

func convertPronunciations(pronunciations []languageconversion.Pronunciation) []api.Pronunciation {
	result := make([]api.Pronunciation, 0, len(pronunciations))
	for _, pronunciation := range pronunciations {
		result = append(result, api.Pronunciation{
			Pronunciation: pronunciation.Text,
			Candidates:    pronunciation.Candidates,
		})
	}
	return result
}
