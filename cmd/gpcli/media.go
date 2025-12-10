package main

import (
	"context"
	"fmt"

	gogpm "github.com/viperadnan-git/gogpm"
	"github.com/viperadnan-git/gogpm/internal/core"

	"github.com/urfave/cli/v3"
)

func deleteAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.NArg() < 1 {
		return fmt.Errorf("item key or file path required")
	}

	if err := loadConfig(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	cfg := cfgManager.GetConfig()

	input := cmd.Args().First()
	restore := cmd.Bool("restore")

	itemKey, err := gogpm.ResolveItemKey(ctx, input)
	if err != nil {
		return err
	}

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

	if restore {
		logger.Info("restoring from trash", "item_key", itemKey)
		if err := apiClient.RestoreFromTrash([]string{itemKey}); err != nil {
			return fmt.Errorf("failed to restore from trash: %w", err)
		}
	} else {
		logger.Info("moving to trash", "item_key", itemKey)
		if err := apiClient.MoveToTrash([]string{itemKey}); err != nil {
			return fmt.Errorf("failed to move to trash: %w", err)
		}
	}

	return nil
}

func archiveAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.NArg() < 1 {
		return fmt.Errorf("item key or file path required")
	}

	if err := loadConfig(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	cfg := cfgManager.GetConfig()

	input := cmd.Args().First()
	unarchive := cmd.Bool("unarchive")

	itemKey, err := gogpm.ResolveItemKey(ctx, input)
	if err != nil {
		return err
	}

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

	isArchived := !unarchive
	if isArchived {
		logger.Info("archiving", "item_key", itemKey)
	} else {
		logger.Info("unarchiving", "item_key", itemKey)
	}

	if err := apiClient.SetArchived([]string{itemKey}, isArchived); err != nil {
		return fmt.Errorf("failed to set archived status: %w", err)
	}

	return nil
}

func favouriteAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.NArg() < 1 {
		return fmt.Errorf("item key or file path required")
	}

	if err := loadConfig(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	cfg := cfgManager.GetConfig()

	input := cmd.Args().First()
	remove := cmd.Bool("remove")

	itemKey, err := gogpm.ResolveItemKey(ctx, input)
	if err != nil {
		return err
	}

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

	isFavourite := !remove
	if isFavourite {
		logger.Info("adding to favourites", "item_key", itemKey)
	} else {
		logger.Info("removing from favourites", "item_key", itemKey)
	}

	if err := apiClient.SetFavourite(itemKey, isFavourite); err != nil {
		return fmt.Errorf("failed to set favourite status: %w", err)
	}

	return nil
}

func captionAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.NArg() < 2 {
		return fmt.Errorf("item key/file path and caption required")
	}

	if err := loadConfig(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	cfg := cfgManager.GetConfig()

	input := cmd.Args().First()
	caption := cmd.Args().Get(1)

	itemKey, err := gogpm.ResolveItemKey(ctx, input)
	if err != nil {
		return err
	}

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

	logger.Info("setting caption", "item_key", itemKey, "caption", caption)

	if err := apiClient.SetCaption(itemKey, caption); err != nil {
		return fmt.Errorf("failed to set caption: %w", err)
	}

	return nil
}
