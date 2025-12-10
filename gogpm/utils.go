package gogpm

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gpcli/gogpm/core"
)

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

	filePath := resolveOutputPath(outputPath, filename)
	if err := writeToFile(filePath, resp.Body); err != nil {
		return "", err
	}
	return filePath, nil
}

// DownloadThumbnail downloads a thumbnail to the specified output path
// Returns the final output path
func DownloadThumbnail(api *core.Api, thumbnailURL, outputPath, mediaKey string) (string, error) {
	req, err := http.NewRequest("GET", thumbnailURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	bearerToken, err := api.BearerToken()
	if err != nil {
		return "", fmt.Errorf("failed to get bearer token: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", bearerToken))
	req.Header.Set("User-Agent", api.UserAgent)
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := api.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	filename := mediaKey + ".jpg"
	filePath := resolveOutputPath(outputPath, filename)
	if err := writeToFile(filePath, resp.Body); err != nil {
		return "", fmt.Errorf("failed to write thumbnail: %w", err)
	}
	return filePath, nil
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
