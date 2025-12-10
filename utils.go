package main

import (
	"fmt"
	"gpcli/src/api"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func downloadFile(apiClient *api.Api, downloadURL, outputPath string) error {
	// Create HTTP request
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Use common headers for authentication
	bearerToken, err := apiClient.BearerToken()
	if err != nil {
		return fmt.Errorf("failed to get bearer token: %w", err)
	}
	headers := apiClient.CommonHeaders(bearerToken)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := apiClient.Client.Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Extract filename from Content-Disposition header
	filename := extractFilenameFromContentDisposition(resp.Header.Get("Content-Disposition"))
	if filename == "" {
		// Fallback to URL extraction
		filename = extractFilenameFromURL(downloadURL)
	}
	if filename == "" {
		filename = "download"
	}

	// Determine the final file path
	var filePath string
	if outputPath == "" {
		// Use current directory with extracted filename
		filePath = filename
	} else {
		// Check if outputPath is a directory
		info, err := os.Stat(outputPath)
		if err == nil && info.IsDir() {
			// It's a directory, use it with the extracted filename
			filePath = filepath.Join(outputPath, filename)
		} else if err != nil && os.IsNotExist(err) {
			// Path doesn't exist, treat it as a file path
			// Ensure parent directory exists
			parentDir := filepath.Dir(outputPath)
			if parentDir != "." && parentDir != "/" {
				if err := os.MkdirAll(parentDir, 0755); err != nil {
					return fmt.Errorf("failed to create directory %s: %w", parentDir, err)
				}
			}
			filePath = outputPath
		} else if err != nil {
			return fmt.Errorf("failed to stat output path: %w", err)
		} else {
			// It exists and is a file - use as the output path
			filePath = outputPath
		}
	}

	slog.Info("downloading", "path", filePath)

	// Create output file
	outFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Copy response body to file
	written, err := io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	slog.Info("downloaded", "bytes", written, "path", filePath)
	return nil
}

func extractFilenameFromContentDisposition(header string) string {
	if header == "" {
		return ""
	}

	// Parse Content-Disposition header
	// Format: attachment; filename="example.jpg" or attachment; filename*=UTF-8''example.jpg
	parts := strings.Split(header, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "filename*=") {
			// RFC 5987 encoded filename (e.g., filename*=UTF-8''example.jpg)
			value := strings.TrimPrefix(part, "filename*=")
			// Extract the actual filename after the encoding prefix
			if idx := strings.Index(value, "''"); idx != -1 {
				filename := value[idx+2:]
				// URL decode the filename
				if decoded, err := url.PathUnescape(filename); err == nil {
					return decoded
				}
				return filename
			}
		} else if strings.HasPrefix(part, "filename=") {
			value := strings.TrimPrefix(part, "filename=")
			// Remove quotes if present
			value = strings.Trim(value, "\"")
			return value
		}
	}
	return ""
}

func extractFilenameFromURL(urlStr string) string {
	// Parse the URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}

	// Get the path and extract the last segment
	pathSegments := strings.Split(parsedURL.Path, "/")
	for i := len(pathSegments) - 1; i >= 0; i-- {
		segment := pathSegments[i]
		if segment != "" {
			// Check if it looks like a filename (has an extension)
			if strings.Contains(segment, ".") {
				return segment
			}
		}
	}

	// If no filename found in path, try to find it in query params or use media key
	return ""
}

func downloadThumbnail(apiClient *api.Api, thumbnailURL, outputPath, mediaKey string) error {
	// Create HTTP request
	req, err := http.NewRequest("GET", thumbnailURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set auth headers
	bearerToken, err := apiClient.BearerToken()
	if err != nil {
		return fmt.Errorf("failed to get bearer token: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", bearerToken))
	req.Header.Set("User-Agent", apiClient.UserAgent)
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := apiClient.Client.Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Default filename is media_key.jpg (since we force JPEG)
	filename := mediaKey + ".jpg"

	// Determine the final file path
	var filePath string
	if outputPath == "" {
		filePath = filename
	} else {
		info, err := os.Stat(outputPath)
		if err == nil && info.IsDir() {
			filePath = filepath.Join(outputPath, filename)
		} else if err != nil && os.IsNotExist(err) {
			parentDir := filepath.Dir(outputPath)
			if parentDir != "." && parentDir != "/" {
				if err := os.MkdirAll(parentDir, 0755); err != nil {
					return fmt.Errorf("failed to create directory %s: %w", parentDir, err)
				}
			}
			filePath = outputPath
		} else if err != nil {
			return fmt.Errorf("failed to stat output path: %w", err)
		} else {
			filePath = outputPath
		}
	}

	slog.Info("downloading thumbnail", "path", filePath)

	outFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	written, err := io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	slog.Info("downloaded thumbnail", "bytes", written, "path", filePath)
	return nil
}
