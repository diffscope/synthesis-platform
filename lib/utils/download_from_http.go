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

package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type DownloadProgressCallback func(totalBytes int64, downloadedBytes int64, remaining time.Duration, speedBytesPerSecond float64)

type downloadMetadata struct {
	URL            string `json:"url"`
	ETag           string `json:"etag"`
	LastModified   string `json:"last_modified"`
	TotalSize      int64  `json:"total_size"`
	DownloadedSize int64  `json:"downloaded_size"`
}

var errResumeNotPossible = errors.New("resume is not possible")

func DownloadFromHttp(urlStr string, dir string, resourceName string, noCache bool, onProgress DownloadProgressCallback) error {
	if strings.TrimSpace(urlStr) == "" {
		return fmt.Errorf("url is empty")
	}
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("dir is empty")
	}
	if strings.TrimSpace(resourceName) == "" {
		return fmt.Errorf("resourceName is empty")
	}
	if _, err := url.ParseRequestURI(urlStr); err != nil {
		return fmt.Errorf("invalid url %q: %w", urlStr, err)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dir %q failed: %w", dir, err)
	}

	finalPath := filepath.Join(dir, resourceName)
	tmpPath := finalPath + ".dsspdltmp"
	metaPath := finalPath + ".dsspdlmeta"

	if noCache {
		_ = os.Remove(tmpPath)
		_ = os.Remove(metaPath)
	}

	meta, hasValidMeta := readMetadata(metaPath, urlStr)

	if !noCache && hasValidMeta {
		if canUseCompleteCache(finalPath, meta) {
			if validateCompleteCache(urlStr, meta) {
				total := meta.TotalSize
				if total < 0 {
					total = meta.DownloadedSize
				}
				reportProgress(onProgress, total, meta.DownloadedSize, 0, 0)
				return nil
			}
		}

		if canResumeFromCache(tmpPath, meta) {
			reportProgress(onProgress, meta.TotalSize, meta.DownloadedSize, 0, 0)
			if err := resumeDownload(urlStr, finalPath, tmpPath, metaPath, meta, onProgress); err == nil {
				return nil
			}
		}
	}

	if err := fullDownload(urlStr, finalPath, tmpPath, metaPath, onProgress); err != nil {
		return err
	}

	return nil
}

func readMetadata(metaPath string, expectedURL string) (downloadMetadata, bool) {
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return downloadMetadata{}, false
	}

	var meta downloadMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return downloadMetadata{}, false
	}

	if meta.URL != expectedURL {
		return downloadMetadata{}, false
	}
	if meta.TotalSize < 0 || meta.DownloadedSize < 0 {
		return downloadMetadata{}, false
	}
	if meta.TotalSize > 0 && meta.DownloadedSize > meta.TotalSize {
		return downloadMetadata{}, false
	}

	return meta, true
}

func canUseCompleteCache(finalPath string, meta downloadMetadata) bool {
	if meta.TotalSize <= 0 {
		return false
	}
	if meta.DownloadedSize != meta.TotalSize {
		return false
	}

	info, err := os.Stat(finalPath)
	if err != nil {
		return false
	}

	return info.Size() == meta.TotalSize
}

func canResumeFromCache(tmpPath string, meta downloadMetadata) bool {
	if meta.TotalSize <= 0 {
		return false
	}
	if meta.ETag == "" && meta.LastModified == "" {
		// Without a validator, we cannot safely prove the remote object is unchanged.
		return false
	}
	if meta.DownloadedSize <= 0 || meta.DownloadedSize >= meta.TotalSize {
		return false
	}

	info, err := os.Stat(tmpPath)
	if err != nil {
		return false
	}

	return info.Size() == meta.DownloadedSize
}

func validateCompleteCache(urlStr string, meta downloadMetadata) bool {
	if meta.ETag == "" && meta.LastModified == "" {
		return false
	}

	req, err := http.NewRequest(http.MethodHead, urlStr, nil)
	if err != nil {
		return false
	}
	if meta.ETag != "" {
		req.Header.Set("If-None-Match", meta.ETag)
	}
	if meta.LastModified != "" {
		req.Header.Set("If-Modified-Since", meta.LastModified)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusNotModified {
		return true
	}

	if resp.StatusCode != http.StatusOK {
		return false
	}

	if meta.ETag != "" {
		etag := resp.Header.Get("ETag")
		if etag == "" || etag != meta.ETag {
			return false
		}
	}

	if meta.LastModified != "" {
		lastModified := resp.Header.Get("Last-Modified")
		if lastModified == "" || lastModified != meta.LastModified {
			return false
		}
	}

	if meta.TotalSize > 0 {
		contentLength := resp.ContentLength
		if contentLength < 0 || contentLength != meta.TotalSize {
			return false
		}
	}

	return true
}

func fullDownload(urlStr string, finalPath string, tmpPath string, metaPath string, onProgress DownloadProgressCallback) error {
	_ = os.Remove(tmpPath)

	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open temp file %q failed: %w", tmpPath, err)
	}
	closed := false
	defer func() {
		if !closed {
			_ = tmpFile.Close()
		}
	}()

	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return fmt.Errorf("create http request failed: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed, status code: %d", resp.StatusCode)
	}

	totalSize := resp.ContentLength
	if totalSize < 0 {
		totalSize = 0
	}

	meta := downloadMetadata{
		URL:            urlStr,
		ETag:           resp.Header.Get("ETag"),
		LastModified:   resp.Header.Get("Last-Modified"),
		TotalSize:      totalSize,
		DownloadedSize: 0,
	}
	_ = writeMetadata(metaPath, meta)

	if err := copyWithProgress(resp.Body, tmpFile, metaPath, &meta, 0, onProgress); err != nil {
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file failed: %w", err)
	}
	closed = true

	if err := finalizeDownload(finalPath, tmpPath); err != nil {
		return err
	}

	meta.DownloadedSize = meta.TotalSize
	if meta.TotalSize <= 0 {
		info, statErr := os.Stat(finalPath)
		if statErr == nil {
			meta.TotalSize = info.Size()
			meta.DownloadedSize = info.Size()
		}
	}
	if err := writeMetadata(metaPath, meta); err != nil {
		return fmt.Errorf("write metadata failed: %w", err)
	}

	reportProgress(onProgress, meta.TotalSize, meta.DownloadedSize, 0, 0)
	return nil
}

func resumeDownload(urlStr string, finalPath string, tmpPath string, metaPath string, meta downloadMetadata, onProgress DownloadProgressCallback) error {
	tmpFile, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return errResumeNotPossible
	}
	closed := false
	defer func() {
		if !closed {
			_ = tmpFile.Close()
		}
	}()

	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return errResumeNotPossible
	}

	req.Header.Set("Range", fmt.Sprintf("bytes=%d-", meta.DownloadedSize))
	if meta.ETag != "" {
		req.Header.Set("If-Range", meta.ETag)
	} else if meta.LastModified != "" {
		req.Header.Set("If-Range", meta.LastModified)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errResumeNotPossible
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusPartialContent {
		return errResumeNotPossible
	}

	contentRange := resp.Header.Get("Content-Range")
	if contentRange == "" {
		return errResumeNotPossible
	}

	rangeStart, totalSize, ok := parseContentRange(contentRange)
	if !ok || rangeStart != meta.DownloadedSize {
		return errResumeNotPossible
	}

	if totalSize > 0 && meta.TotalSize > 0 && totalSize != meta.TotalSize {
		return errResumeNotPossible
	}

	if totalSize > 0 {
		meta.TotalSize = totalSize
	}

	if err := copyWithProgress(resp.Body, tmpFile, metaPath, &meta, meta.DownloadedSize, onProgress); err != nil {
		return err
	}

	if meta.TotalSize > 0 && meta.DownloadedSize != meta.TotalSize {
		return errResumeNotPossible
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file failed: %w", err)
	}
	closed = true

	if err := finalizeDownload(finalPath, tmpPath); err != nil {
		return err
	}

	if meta.TotalSize <= 0 {
		info, statErr := os.Stat(finalPath)
		if statErr != nil {
			return fmt.Errorf("stat final file failed: %w", statErr)
		}
		meta.TotalSize = info.Size()
		meta.DownloadedSize = info.Size()
	}

	if err := writeMetadata(metaPath, meta); err != nil {
		return fmt.Errorf("write metadata failed: %w", err)
	}

	reportProgress(onProgress, meta.TotalSize, meta.DownloadedSize, 0, 0)
	return nil
}

func copyWithProgress(reader io.Reader, writer io.Writer, metaPath string, meta *downloadMetadata, initialDownloaded int64, onProgress DownloadProgressCallback) error {
	const (
		bufferSize            = 64 * 1024
		reportInterval        = 200 * time.Millisecond
		metadataFlushInterval = 1 * time.Second
	)

	buf := make([]byte, bufferSize)
	downloaded := initialDownloaded
	start := time.Now()
	lastReport := start
	lastMetaFlush := start

	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			if _, err := writer.Write(buf[:n]); err != nil {
				return fmt.Errorf("write temp file failed: %w", err)
			}

			downloaded += int64(n)
			meta.DownloadedSize = downloaded

			now := time.Now()
			if now.Sub(lastReport) >= reportInterval {
				reportProgress(onProgress, meta.TotalSize, downloaded, now.Sub(start), speedBytesPerSecond(downloaded-initialDownloaded, now.Sub(start)))
				lastReport = now
			}

			if now.Sub(lastMetaFlush) >= metadataFlushInterval {
				if err := writeMetadata(metaPath, *meta); err != nil {
					return fmt.Errorf("write metadata failed: %w", err)
				}
				lastMetaFlush = now
			}
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return fmt.Errorf("read download stream failed: %w", readErr)
		}
	}

	if err := writeMetadata(metaPath, *meta); err != nil {
		return fmt.Errorf("write metadata failed: %w", err)
	}

	elapsed := time.Since(start)
	reportProgress(onProgress, meta.TotalSize, downloaded, elapsed, speedBytesPerSecond(downloaded-initialDownloaded, elapsed))
	return nil
}

func speedBytesPerSecond(downloaded int64, elapsed time.Duration) float64 {
	if downloaded <= 0 || elapsed <= 0 {
		return 0
	}
	return float64(downloaded) / elapsed.Seconds()
}

func reportProgress(onProgress DownloadProgressCallback, totalBytes int64, downloadedBytes int64, elapsed time.Duration, speed float64) {
	if onProgress == nil {
		return
	}

	remaining := time.Duration(0)
	if totalBytes > 0 && downloadedBytes < totalBytes && speed > 0 {
		remainingSeconds := float64(totalBytes-downloadedBytes) / speed
		remaining = time.Duration(remainingSeconds * float64(time.Second))
	}

	onProgress(totalBytes, downloadedBytes, remaining, speed)
}

func writeMetadata(metaPath string, meta downloadMetadata) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}

	tmpMetaPath := metaPath + ".tmp"
	if err := os.WriteFile(tmpMetaPath, data, 0o644); err != nil {
		return err
	}

	if err := os.Rename(tmpMetaPath, metaPath); err != nil {
		if removeErr := os.Remove(metaPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			_ = os.Remove(tmpMetaPath)
			return removeErr
		}
		if err2 := os.Rename(tmpMetaPath, metaPath); err2 != nil {
			_ = os.Remove(tmpMetaPath)
			return err2
		}
	}

	return nil
}

func parseContentRange(contentRange string) (start int64, total int64, ok bool) {
	// Example: bytes 100-999/1000
	parts := strings.SplitN(strings.TrimSpace(contentRange), " ", 2)
	if len(parts) != 2 || parts[0] != "bytes" {
		return 0, 0, false
	}

	rangeAndTotal := strings.SplitN(parts[1], "/", 2)
	if len(rangeAndTotal) != 2 {
		return 0, 0, false
	}

	if rangeAndTotal[1] == "*" {
		return 0, 0, false
	}

	totalSize, err := strconv.ParseInt(rangeAndTotal[1], 10, 64)
	if err != nil || totalSize <= 0 {
		return 0, 0, false
	}

	rangeParts := strings.SplitN(rangeAndTotal[0], "-", 2)
	if len(rangeParts) != 2 {
		return 0, 0, false
	}

	startPos, err := strconv.ParseInt(rangeParts[0], 10, 64)
	if err != nil || startPos < 0 {
		return 0, 0, false
	}

	return startPos, totalSize, true
}

func finalizeDownload(finalPath string, tmpPath string) error {
	if err := os.Remove(finalPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove old file %q failed: %w", finalPath, err)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		return fmt.Errorf("rename temp file failed: %w", err)
	}

	return nil
}
