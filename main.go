package gpm

import (
	"fmt"
	"sync"

	"github.com/viperadnan-git/go-gpm/internal/core"
)

// LibraryStateResponse contains raw response data from library state APIs
type LibraryStateResponse = core.LibraryStateResponse

// MediaItem represents a media item in the library
type MediaItem = core.MediaItem

// MediaItemCopy represents a copy of a media item (e.g., shared to an album)
type MediaItemCopy = core.MediaItemCopy

// AlbumItem represents an album in the library
type AlbumItem = core.AlbumItem

// AlbumMediaItem represents a media item inside an album
type AlbumMediaItem = core.AlbumMediaItem

// ApiConfig holds configuration for the Google Photos API client
type ApiConfig = core.ApiConfig

// GooglePhotosAPI is the main API client for Google Photos operations
type GooglePhotosAPI struct {
	*core.Api
	uploadMu sync.Mutex // Serializes upload batches
}

// NewGooglePhotosAPI creates a new Google Photos API client
func NewGooglePhotosAPI(cfg ApiConfig) (*GooglePhotosAPI, error) {
	coreApi, err := core.NewApi(cfg)
	if err != nil {
		return nil, err
	}
	return &GooglePhotosAPI{Api: coreApi}, nil
}

// DownloadThumbnail downloads a thumbnail to the specified output path
// Returns the final output path
func (g *GooglePhotosAPI) DownloadThumbnail(mediaKey string, width, height int, forceJpeg, noOverlay bool, outputPath string) (string, error) {
	body, err := g.GetThumbnail(mediaKey, width, height, forceJpeg, noOverlay)
	if err != nil {
		return "", err
	}
	defer body.Close()

	filename := mediaKey + ".jpg"
	return DownloadFromReader(body, outputPath, filename)
}

// DownloadMedia downloads a media item to the specified output path
// Returns the final output path
func (g *GooglePhotosAPI) DownloadMedia(mediaKey string, outputPath string) (string, error) {
	downloadURL, _, err := g.GetDownloadUrl(mediaKey)
	if err != nil {
		return "", err
	}
	if downloadURL == "" {
		return "", fmt.Errorf("no download URL available")
	}
	return DownloadFile(downloadURL, outputPath)
}

// GetLibraryState fetches the library state using a state token
// Returns raw protobuf bytes for inspection
func (g *GooglePhotosAPI) GetLibraryState(stateToken string) (*LibraryStateResponse, error) {
	return g.Api.GetLibraryState(stateToken)
}

// GetLibraryPageInit fetches initial library page using a page token
// Returns raw protobuf bytes for inspection
func (g *GooglePhotosAPI) GetLibraryPageInit(pageToken string) (*LibraryStateResponse, error) {
	return g.Api.GetLibraryPageInit(pageToken)
}

// GetLibraryPage fetches library page using both page and state tokens
// Returns raw protobuf bytes for inspection
func (g *GooglePhotosAPI) GetLibraryPage(pageToken, stateToken string) (*LibraryStateResponse, error) {
	return g.Api.GetLibraryPage(pageToken, stateToken)
}
