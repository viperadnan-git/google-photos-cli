package main

import (
	"context"
	"encoding/hex"
	"fmt"

	gpm "github.com/viperadnan-git/go-gpm"

	"github.com/urfave/cli/v3"
)

func libraryTestAction(ctx context.Context, cmd *cli.Command) error {
	if err := loadConfig(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	cfg := cfgManager.GetConfig()

	stateToken := cmd.String("state-token")
	pageToken := cmd.String("page-token")
	raw := cmd.Bool("raw")

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

	var resp *gpm.LibraryStateResponse
	var funcName string

	if stateToken != "" && pageToken != "" {
		logger.Info("calling GetLibraryPage", "state_token", truncate(stateToken, 20), "page_token", truncate(pageToken, 20))
		funcName = "GetLibraryPage"
		resp, err = apiClient.GetLibraryPage(pageToken, stateToken)
	} else if pageToken != "" {
		logger.Info("calling GetLibraryPageInit", "page_token", truncate(pageToken, 20))
		funcName = "GetLibraryPageInit"
		resp, err = apiClient.GetLibraryPageInit(pageToken)
	} else {
		logger.Info("calling GetLibraryState", "state_token", truncate(stateToken, 20))
		funcName = "GetLibraryState"
		resp, err = apiClient.GetLibraryState(stateToken)
	}

	if err != nil {
		return fmt.Errorf("%s failed: %w", funcName, err)
	}

	logger.Info("response received", "size_bytes", len(resp.RawBytes))

	if raw {
		// Print raw hex dump
		fmt.Printf("\n=== Raw Protobuf Bytes (%d bytes) ===\n", len(resp.RawBytes))
		fmt.Println(hex.Dump(resp.RawBytes))
	} else if cmd.Bool("raw-json") {
		// Print raw protobuf as JSON (for debugging structure)
		jsonStr, err := resp.ToRawJSON()
		if err != nil {
			return fmt.Errorf("failed to parse protobuf: %w", err)
		}
		fmt.Printf("\n=== Raw Protobuf JSON ===\n")
		fmt.Println(jsonStr)
	} else {
		// Print parsed JSON
		jsonStr, err := resp.ToJSON()
		if err != nil {
			logger.Warn("failed to parse protobuf to JSON, falling back to hex dump", "error", err)
			fmt.Printf("\n=== Raw Protobuf Bytes (%d bytes) ===\n", len(resp.RawBytes))
			fmt.Println(hex.Dump(resp.RawBytes))
		} else {
			fmt.Printf("\n=== Parsed Protobuf Response ===\n")
			fmt.Println(jsonStr)
		}
	}

	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
