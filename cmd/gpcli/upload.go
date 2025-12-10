package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	gogpm "github.com/viperadnan-git/gogpm"
	"github.com/viperadnan-git/gogpm/internal/core"

	"github.com/urfave/cli/v3"
)

func uploadAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.NArg() < 1 {
		return fmt.Errorf("filepath required")
	}

	filePath := cmd.Args().First()

	// Validate that filepath exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file or directory does not exist: %s", filePath)
	}

	// Load config
	if err := loadConfig(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	cfg := cfgManager.GetConfig()

	// Get CLI flags
	threads := int(cmd.Int("threads"))
	quality := cmd.String("quality")
	if quality != "original" && quality != "storage-saver" {
		return fmt.Errorf("invalid quality: %s (use 'original' or 'storage-saver')", quality)
	}
	albumName := cmd.String("album")

	// Build upload options from CLI flags
	uploadOpts := gogpm.UploadOptions{
		Threads:         threads,
		Recursive:       cmd.Bool("recursive"),
		ForceUpload:     cmd.Bool("force"),
		DeleteFromHost:  cmd.Bool("delete"),
		DisableFilter:   cmd.Bool("disable-filter"),
		Caption:         cmd.String("caption"),
		ShouldFavourite: cmd.Bool("favourite"),
		ShouldArchive:   cmd.Bool("archive"),
		Quality:         quality,
		UseQuota:        cmd.Bool("use-quota"),
	}

	// Resolve auth data
	authData := getAuthData(cfg)
	if authData == "" {
		return fmt.Errorf("no authentication configured. Use 'gpcli auth add' to add credentials")
	}

	// Build API config
	apiCfg := core.ApiConfig{
		AuthData: authData,
		Proxy:    cfg.Proxy,
	}

	// Log start
	logger.Info("scanning files", "path", filePath)

	// Track results
	var mu sync.Mutex
	var totalFiles int
	var uploaded int
	var existing int
	var failed int
	var successfulMediaKeys []string
	done := make(chan struct{})

	// Create event callback
	eventCallback := func(event string, data any) {
		mu.Lock()
		defer mu.Unlock()

		switch event {
		case "uploadStart":
			if start, ok := data.(gogpm.UploadBatchStart); ok {
				totalFiles = start.Total
				logger.Info("starting upload", "files", totalFiles, "threads", threads)
			}
		case "ThreadStatus":
			if status, ok := data.(gogpm.ThreadStatus); ok {
				// Only log active upload states at debug level
				if status.Status == "uploading" || status.Status == "hashing" {
					logger.Debug(status.Message, "file", status.FileName)
				}
			}
		case "FileStatus":
			if result, ok := data.(gogpm.FileUploadResult); ok {
				processed := uploaded + existing + failed + 1
				progress := fmt.Sprintf("[%d/%d]", processed, totalFiles)

				if result.IsError {
					failed++
					logger.Error(progress+" failed", "file", result.Path, "error", result.Error)
				} else if result.IsExisting {
					existing++
					logger.Debug(progress+" skipped (exists)", "file", result.Path)
					if result.MediaKey != "" {
						successfulMediaKeys = append(successfulMediaKeys, result.MediaKey)
					}
				} else {
					uploaded++
					logger.Debug(progress+" uploaded", "file", result.Path)
					if result.MediaKey != "" {
						successfulMediaKeys = append(successfulMediaKeys, result.MediaKey)
					}
				}
			}
		case "uploadStop":
			close(done)
		}
	}

	api, err := gogpm.NewGooglePhotosAPI(apiCfg)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	// Run upload in background
	go func() {
		api.Upload([]string{filePath}, uploadOpts, eventCallback)
	}()

	// Wait for upload to complete
	<-done

	// Print summary
	logger.Info("upload complete", "uploaded", uploaded, "skipped", existing, "failed", failed)

	// Handle album creation if album name was specified
	if albumName != "" && len(successfulMediaKeys) > 0 {
		logger.Info("adding to album", "album", albumName)

		const batchSize = 500
		firstBatchEnd := min(batchSize, len(successfulMediaKeys))

		albumMediaKey, err := api.CreateAlbum(albumName, successfulMediaKeys[:firstBatchEnd])
		if err != nil {
			return fmt.Errorf("failed to create album: %w", err)
		}

		for i := batchSize; i < len(successfulMediaKeys); i += batchSize {
			end := min(i+batchSize, len(successfulMediaKeys))
			if err = api.AddMediaToAlbum(albumMediaKey, successfulMediaKeys[i:end]); err != nil {
				logger.Warn("failed to add batch to album", "error", err)
			}
		}

		logger.Info("album ready", "album", albumName, "items", len(successfulMediaKeys))
	}

	return nil
}
