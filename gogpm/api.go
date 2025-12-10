package gogpm

import (
	"gpcli/gogpm/core"
	"sync"
)

// GooglePhotosAPI is the main API client for Google Photos operations
type GooglePhotosAPI struct {
	*core.Api
	wg      sync.WaitGroup
	cancel  chan struct{}
	running bool
}

// NewGooglePhotosAPI creates a new Google Photos API client
func NewGooglePhotosAPI(cfg core.ApiConfig) (*GooglePhotosAPI, error) {
	coreApi, err := core.NewApi(cfg)
	if err != nil {
		return nil, err
	}
	return &GooglePhotosAPI{Api: coreApi}, nil
}

// IsRunning returns true if an upload operation is currently running
func (g *GooglePhotosAPI) IsRunning() bool {
	return g.running
}

// Cancel cancels the current upload operation
func (g *GooglePhotosAPI) Cancel() {
	if g.cancel != nil {
		close(g.cancel)
		g.cancel = nil
	}
}
