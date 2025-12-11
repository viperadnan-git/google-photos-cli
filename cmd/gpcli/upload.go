package main

import (
	"context"
	"fmt"
	"os"

	gogpm "github.com/viperadnan-git/gogpm"

	"github.com/urfave/cli/v3"
)

func uploadAction(ctx context.Context, cmd *cli.Command) error {
	filePath := cmd.StringArg("filepath")

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
	if threads == 0 { threads = cfg.UploadThreads }
	quality := cmd.String("quality")
	if quality == "" { quality = cfg.Quality }
	if quality != "original" && quality != "storage-saver" {
		return fmt.Errorf("invalid quality: %s (use 'original' or 'storage-saver')", quality)
	}
	albumName := cmd.String("album")

	// Build upload options from CLI flags
	uploadOpts := gogpm.UploadOptions{
		Workers:         threads,
		Recursive:       cmd.Bool("recursive"),
		ForceUpload:     cmd.Bool("force"),
		DeleteFromHost:  cmd.Bool("delete"),
		DisableFilter:   cmd.Bool("disable-filter"),
		Caption:         cmd.String("caption"),
		ShouldFavourite: cmd.Bool("favourite"),
		ShouldArchive:   cmd.Bool("archive"),
		Quality:         quality,
		UseQuota:        cmd.Bool("use-quota") || cfg.UseQuota,
	}

	// Resolve auth data
	authData := getAuthData(cfg)
	if authData == "" {
		return fmt.Errorf("no authentication configured. Use 'gpcli auth add' to add credentials")
	}

	// Build API config
	apiCfg := gogpm.ApiConfig{
		AuthData: authData,
		Proxy:    cfg.Proxy,
	}

	// Log start
	logger.Info("scanning files", "path", filePath)

	api, err := gogpm.NewGooglePhotosAPI(apiCfg)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	// Track results
	var totalFiles, uploaded, existing, failed int
	var successfulMediaKeys []string

	// Process upload events
	for event := range api.Upload(ctx, []string{filePath}, uploadOpts) {
		if event.Total > 0 {
			totalFiles = event.Total
			logger.Info("starting upload", "files", totalFiles, "threads", threads)
		}

		switch event.Status {
		case gogpm.StatusHashing, gogpm.StatusUploading:
			logger.Debug(string(event.Status), "file", event.Path)
		case gogpm.StatusCompleted:
			uploaded++
			progress := fmt.Sprintf("[%d/%d]", uploaded+existing+failed, totalFiles)
			logger.Info(progress+" uploaded", "mediaKey", event.MediaKey, "file", event.Path)
			if event.MediaKey != "" {
				successfulMediaKeys = append(successfulMediaKeys, event.MediaKey)
			}
		case gogpm.StatusSkipped:
			existing++
			progress := fmt.Sprintf("[%d/%d]", uploaded+existing+failed, totalFiles)
			logger.Info(progress+" skipped", "mediaKey", event.MediaKey, "file", event.Path, "exists", true)
			if event.MediaKey != "" {
				successfulMediaKeys = append(successfulMediaKeys, event.MediaKey)
			}
		case gogpm.StatusFailed:
			failed++
			progress := fmt.Sprintf("[%d/%d]", uploaded+existing+failed, totalFiles)
			logger.Error(progress+" failed", "file", event.Path, "error", event.Error)
		}
	}

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
