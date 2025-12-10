package main

import (
	"context"
	"fmt"

	"github.com/creativeprojects/go-selfupdate"
	"github.com/urfave/cli/v3"
	gogpm "github.com/viperadnan-git/gogpm"
)

func upgradeAction(ctx context.Context, cmd *cli.Command) error {
	// Get target version (empty string = latest)
	targetVersion := ""
	if cmd.NArg() > 0 {
		targetVersion = cmd.Args().First()
	}
	checkOnly := cmd.Bool("check")

	// Configure updater for GitHub
	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return fmt.Errorf("failed to create GitHub source: %w", err)
	}

	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source:    source,
		Validator: &selfupdate.ChecksumValidator{UniqueFilename: "checksums.txt"},
	})
	if err != nil {
		return fmt.Errorf("failed to create updater: %w", err)
	}

	repo := selfupdate.ParseSlug("viperadnan-git/gogpm")

	var release *selfupdate.Release
	var found bool

	if targetVersion != "" {
		// Find specific version
		logger.Info("checking for version", "version", targetVersion)
		release, found, err = updater.DetectVersion(ctx, repo, targetVersion)
	} else {
		// Find latest version
		logger.Info("checking for latest version")
		release, found, err = updater.DetectLatest(ctx, repo)
	}

	if err != nil {
		return fmt.Errorf("failed to detect version: %w", err)
	}
	if !found {
		if targetVersion != "" {
			return fmt.Errorf("version %s not found", targetVersion)
		}
		logger.Info("no release found")
		return nil
	}

	currentVersion := gogpm.Version

	// Compare versions (skip if same and no specific version requested)
	if targetVersion == "" && release.Version() == currentVersion {
		logger.Info("already at latest version", "version", currentVersion)
		return nil
	}

	// Check-only mode: display info and exit
	if checkOnly {
		logger.Info("update available", "current", currentVersion, "available", release.Version())
		return nil
	}

	logger.Info("updating", "from", currentVersion, "to", release.Version())

	// Get executable path
	exe, err := selfupdate.ExecutablePath()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Perform update
	if err := updater.UpdateTo(ctx, release, exe); err != nil {
		return fmt.Errorf("failed to update: %w", err)
	}

	logger.Info("successfully updated", "version", release.Version())
	return nil
}
