package api

import (
	"fmt"
	"io"
	"net/http"
)

// GetThumbnailURL builds the thumbnail URL for a media item
func (a *Api) GetThumbnailURL(mediaKey string, width, height int, forceJpeg, noOverlay bool) string {
	url := fmt.Sprintf("https://ap2.googleusercontent.com/gpa/%s=k-sg", mediaKey)
	if width > 0 {
		url += fmt.Sprintf("-w%d", width)
	}
	if height > 0 {
		url += fmt.Sprintf("-h%d", height)
	}
	if forceJpeg {
		url += "-rj"
	}
	if noOverlay {
		url += "-no"
	}
	return url
}

// GetThumbnail downloads the thumbnail bytes for a media item
func (a *Api) GetThumbnail(mediaKey string, width, height int, forceJpeg, noOverlay bool) ([]byte, error) {
	url := a.GetThumbnailURL(mediaKey, width, height, forceJpeg, noOverlay)

	bearerToken, err := a.BearerToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get bearer token: %w", err)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", bearerToken))
	req.Header.Set("User-Agent", a.UserAgent)
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := a.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}
