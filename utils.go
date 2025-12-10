package gogpm

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/viperadnan-git/gogpm/internal/core"
)

// DownloadFromReader saves data from an io.Reader to the specified output path
// Returns the final output path
func DownloadFromReader(reader io.Reader, outputPath, filename string) (string, error) {
	filePath := resolveOutputPath(outputPath, filename)
	if err := writeToFile(filePath, reader); err != nil {
		return "", err
	}
	return filePath, nil
}

// DownloadFile downloads a file from the given URL to the specified output path
// Returns the final output path
func DownloadFile(downloadURL, outputPath string) (string, error) {
	resp, err := http.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	filename := extractFilenameFromContentDisposition(resp.Header.Get("Content-Disposition"))
	if filename == "" {
		filename = extractFilenameFromURL(downloadURL)
	}
	if filename == "" {
		filename = "download"
	}

	return DownloadFromReader(resp.Body, outputPath, filename)
}

// extractFilenameFromContentDisposition extracts filename from Content-Disposition header
func extractFilenameFromContentDisposition(header string) string {
	if header == "" {
		return ""
	}

	parts := strings.Split(header, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "filename*=") {
			value := strings.TrimPrefix(part, "filename*=")
			if idx := strings.Index(value, "''"); idx != -1 {
				filename := value[idx+2:]
				if decoded, err := url.PathUnescape(filename); err == nil {
					return decoded
				}
				return filename
			}
		} else if strings.HasPrefix(part, "filename=") {
			value := strings.TrimPrefix(part, "filename=")
			value = strings.Trim(value, "\"")
			return value
		}
	}
	return ""
}

// extractFilenameFromURL extracts filename from URL path
func extractFilenameFromURL(urlStr string) string {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}

	pathSegments := strings.Split(parsedURL.Path, "/")
	for i := len(pathSegments) - 1; i >= 0; i-- {
		segment := pathSegments[i]
		if segment != "" && strings.Contains(segment, ".") {
			return segment
		}
	}
	return ""
}

// resolveOutputPath determines the final file path based on output path and filename
func resolveOutputPath(outputPath, filename string) string {
	if outputPath == "" {
		return filename
	}

	info, err := os.Stat(outputPath)
	if err == nil && info.IsDir() {
		return filepath.Join(outputPath, filename)
	} else if err != nil && os.IsNotExist(err) {
		parentDir := filepath.Dir(outputPath)
		if parentDir != "." && parentDir != "/" {
			os.MkdirAll(parentDir, 0755)
		}
		return outputPath
	}
	return outputPath
}

// writeToFile writes data from reader to file
func writeToFile(filePath string, reader io.Reader) error {
	outFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, reader)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	return nil
}

// ResolveItemKey resolves input to an item key (dedupKey for file paths)
// If input is a file path (file exists on disk), calculates SHA1 and converts to dedup key
// Otherwise returns input as-is (assumed to be mediaKey or dedupKey)
// Use this for APIs that accept both mediaKey and dedupKey (delete, archive, favourite, caption)
func ResolveItemKey(ctx context.Context, input string) (string, error) {
	// Check if input is a file path by trying to stat it
	if _, err := os.Stat(input); err == nil {
		// File exists, calculate SHA1 and convert to dedup key
		hash, err := CalculateSHA1(ctx, input)
		if err != nil {
			return "", fmt.Errorf("failed to calculate SHA1: %w", err)
		}
		return core.SHA1ToDedupeKey(hash), nil
	}
	// Assume it's already a media key or dedup key
	return input, nil
}

// ResolveMediaKey resolves input to a mediaKey
// If input is a file path, calculates SHA1 and looks up mediaKey via API
// Otherwise returns input as-is (assumed to be mediaKey)
// Use this for APIs that require mediaKey (thumbnail, download)
func ResolveMediaKey(ctx context.Context, apiClient *core.Api, input string) (string, error) {
	// Check if input is a file path by trying to stat it
	if _, err := os.Stat(input); err == nil {
		// File exists, calculate SHA1 and look up mediaKey
		hash, err := CalculateSHA1(ctx, input)
		if err != nil {
			return "", fmt.Errorf("failed to calculate SHA1: %w", err)
		}
		mediaKey, err := apiClient.FindRemoteMediaByHash(hash)
		if err != nil {
			return "", fmt.Errorf("failed to find media in library: %w", err)
		}
		if mediaKey == "" {
			return "", fmt.Errorf("file not found in Google Photos library")
		}
		return mediaKey, nil
	}
	// Assume it's already a media key
	return input, nil
}
