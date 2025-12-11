package core

import (
	"fmt"
	"io"
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

// GetThumbnail returns a streaming response body for the thumbnail
// Caller is responsible for closing the returned ReadCloser
func (a *Api) GetThumbnail(mediaKey string, width, height int, forceJpeg, noOverlay bool) (io.ReadCloser, error) {
	url := a.GetThumbnailURL(mediaKey, width, height, forceJpeg, noOverlay)

	_, resp, err := a.DoRequest(
		url,
		nil,
		WithMethod("GET"),
		WithAuth(),
		WithStatusCheck(),
		WithStreamingResponse(),
	)
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}
