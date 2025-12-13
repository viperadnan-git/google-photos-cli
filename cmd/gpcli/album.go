package main

import (
	"context"
	"fmt"

	gpm "github.com/viperadnan-git/go-gpm"

	"github.com/urfave/cli/v3"
)

func albumCreateAction(ctx context.Context, cmd *cli.Command) error {
	if err := loadConfig(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	cfg := cfgManager.GetConfig()

	albumName := cmd.StringArg("name")
	if albumName == "" {
		return fmt.Errorf("album name is required")
	}

	authData := getAuthData(cfg)
	if authData == "" {
		return fmt.Errorf("no authentication configured. Use 'gpcli auth add' to add credentials")
	}

	apiClient, err := gpm.NewGooglePhotosAPI(gpm.ApiConfig{
		AuthData: authData,
		Proxy:    cfg.Proxy,
	})
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	// Get media items from command-line args
	// Note: cmd.Args().Slice() returns unconsumed args (album name is already consumed by StringArg)
	mediaInputs := cmd.Args().Slice()

	// Resolve all media keys (if any provided)
	var mediaKeys []string
	if len(mediaInputs) > 0 {
		logger.Info("resolving media items", "count", len(mediaInputs))
		mediaKeys = make([]string, 0, len(mediaInputs))
		for _, input := range mediaInputs {
			mediaKey, err := apiClient.ResolveMediaKey(ctx, input)
			if err != nil {
				return fmt.Errorf("failed to resolve media key for %s: %w", input, err)
			}
			mediaKeys = append(mediaKeys, mediaKey)
		}
	}

	logger.Info("creating album", "name", albumName, "media_count", len(mediaKeys))

	albumMediaKey, err := apiClient.CreateAlbum(albumName, mediaKeys)
	if err != nil {
		return fmt.Errorf("failed to create album: %w", err)
	}

	logger.Info("album created successfully", "name", albumName, "album_key", albumMediaKey, "media_count", len(mediaKeys))
	return nil
}

func albumAddAction(ctx context.Context, cmd *cli.Command) error {
	if err := loadConfig(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	cfg := cfgManager.GetConfig()

	albumMediaKey := cmd.StringArg("album-key")
	if albumMediaKey == "" {
		return fmt.Errorf("album media key is required")
	}

	// Collect media inputs from both command-line args and file
	var mediaInputs []string

	// Get all unconsumed arguments (album-key is already consumed by StringArg)
	allArgs := cmd.Args().Slice()
	if len(allArgs) > 0 {
		mediaInputs = append(mediaInputs, allArgs...)
	}

	// Get media items from file if --from-file is provided
	if fromFile := cmd.String("from-file"); fromFile != "" {
		fileInputs, err := readLinesFromFile(fromFile)
		if err != nil {
			return err
		}
		mediaInputs = append(mediaInputs, fileInputs...)
	}

	if len(mediaInputs) == 0 {
		return fmt.Errorf("at least one media item is required (provide via command-line or --from-file)")
	}

	authData := getAuthData(cfg)
	if authData == "" {
		return fmt.Errorf("no authentication configured. Use 'gpcli auth add' to add credentials")
	}

	apiClient, err := gpm.NewGooglePhotosAPI(gpm.ApiConfig{
		AuthData: authData,
		Proxy:    cfg.Proxy,
	})
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	logger.Info("resolving media items", "count", len(mediaInputs))
	mediaKeys := make([]string, 0, len(mediaInputs))
	for _, input := range mediaInputs {
		mediaKey, err := apiClient.ResolveMediaKey(ctx, input)
		if err != nil {
			return fmt.Errorf("failed to resolve media key for %s: %w", input, err)
		}
		mediaKeys = append(mediaKeys, mediaKey)
	}

	logger.Info("adding media to album", "album_key", albumMediaKey, "media_count", len(mediaKeys))

	if err := apiClient.AddMediaToAlbum(albumMediaKey, mediaKeys); err != nil {
		return fmt.Errorf("failed to add media to album: %w", err)
	}

	logger.Info("successfully added media to album", "album_key", albumMediaKey, "media_count", len(mediaKeys))
	return nil
}

func albumDeleteAction(ctx context.Context, cmd *cli.Command) error {
	if err := loadConfig(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	cfg := cfgManager.GetConfig()

	albumMediaKey := cmd.StringArg("album-key")
	if albumMediaKey == "" {
		return fmt.Errorf("album media key is required")
	}

	authData := getAuthData(cfg)
	if authData == "" {
		return fmt.Errorf("no authentication configured. Use 'gpcli auth add' to add credentials")
	}

	apiClient, err := gpm.NewGooglePhotosAPI(gpm.ApiConfig{
		AuthData: authData,
		Proxy:    cfg.Proxy,
	})
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	logger.Info("deleting album", "album_key", albumMediaKey)

	if err := apiClient.DeleteAlbum(albumMediaKey); err != nil {
		return fmt.Errorf("failed to delete album: %w", err)
	}

	logger.Info("album deleted successfully", "album_key", albumMediaKey)
	return nil
}

func albumRenameAction(ctx context.Context, cmd *cli.Command) error {
	if err := loadConfig(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	cfg := cfgManager.GetConfig()

	albumMediaKey := cmd.StringArg("album-key")
	if albumMediaKey == "" {
		return fmt.Errorf("album media key is required")
	}

	newName := cmd.StringArg("new-name")
	if newName == "" {
		return fmt.Errorf("new album name is required")
	}

	authData := getAuthData(cfg)
	if authData == "" {
		return fmt.Errorf("no authentication configured. Use 'gpcli auth add' to add credentials")
	}

	apiClient, err := gpm.NewGooglePhotosAPI(gpm.ApiConfig{
		AuthData: authData,
		Proxy:    cfg.Proxy,
	})
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	logger.Info("renaming album", "album_key", albumMediaKey, "new_name", newName)

	if err := apiClient.RenameAlbum(albumMediaKey, newName); err != nil {
		return fmt.Errorf("failed to rename album: %w", err)
	}

	logger.Info("album renamed successfully", "album_key", albumMediaKey, "new_name", newName)
	return nil
}
