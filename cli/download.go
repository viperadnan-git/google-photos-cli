package cli

import (
	"context"
	"fmt"
	"gpcli/gogpm"
	"gpcli/gogpm/core"

	"github.com/urfave/cli/v3"
)

func downloadAction(ctx context.Context, cmd *cli.Command) error {
	if err := loadConfig(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	cfg := cfgManager.GetConfig()

	input := cmd.Args().First()
	if input == "" {
		return fmt.Errorf("item key or file path required")
	}

	urlOnly := cmd.Bool("url")
	outputPath := cmd.String("output")

	authData := getAuthData(cfg)
	if authData == "" {
		return fmt.Errorf("no authentication configured. Use 'gpcli auth add' to add credentials")
	}

	apiClient, err := core.NewApi(core.ApiConfig{
		AuthData: authData,
		Proxy:    cfg.Proxy,
	})
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	mediaKey, err := gogpm.ResolveMediaKey(ctx, apiClient, input)
	if err != nil {
		return err
	}

	if !urlOnly {
		logger.Info("fetching download URL", "media_key", mediaKey)
	}

	downloadURL, isEdited, err := apiClient.GetDownloadUrl(mediaKey)
	if err != nil {
		return fmt.Errorf("failed to get download URL: %w", err)
	}

	if downloadURL == "" {
		return fmt.Errorf("no download URL available")
	}

	// If --url flag is set, just print the URL and exit
	if urlOnly {
		fmt.Println(downloadURL)
		return nil
	}

	// Download the file
	logger.Info("downloading", "is_edited", isEdited)
	savedPath, err := gogpm.DownloadFile(downloadURL, outputPath)
	if err != nil {
		return err
	}
	logger.Info("download complete", "path", savedPath)
	return nil
}

func thumbnailAction(ctx context.Context, cmd *cli.Command) error {
	if err := loadConfig(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	cfg := cfgManager.GetConfig()

	input := cmd.Args().First()
	if input == "" {
		return fmt.Errorf("item key or file path required")
	}

	outputPath := cmd.String("output")
	width := int(cmd.Int("width"))
	height := int(cmd.Int("height"))
	forceJpeg := cmd.Bool("jpeg")
	noOverlay := !cmd.Bool("overlay")

	authData := getAuthData(cfg)
	if authData == "" {
		return fmt.Errorf("no authentication configured. Use 'gpcli auth add' to add credentials")
	}

	apiClient, err := core.NewApi(core.ApiConfig{
		AuthData: authData,
		Proxy:    cfg.Proxy,
	})
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	mediaKey, err := gogpm.ResolveMediaKey(ctx, apiClient, input)
	if err != nil {
		return err
	}

	logger.Info("downloading thumbnail", "media_key", mediaKey)

	body, err := apiClient.GetThumbnail(mediaKey, width, height, forceJpeg, noOverlay)
	if err != nil {
		return err
	}
	defer body.Close()

	filename := mediaKey + ".jpg"
	savedPath, err := gogpm.DownloadFromReader(body, outputPath, filename)
	if err != nil {
		return err
	}
	logger.Info("thumbnail saved", "path", savedPath)
	return nil
}
