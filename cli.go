package main

import (
	"fmt"
	"gpcli/src"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/urfave/cli/v2"
)

// parseLogLevel converts a string log level to slog.Level
func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func runCLI() {
	app := &cli.App{
		Name:                   "gpcli",
		Usage:                  "Google Photos unofficial CLI client",
		Version:                src.Version,
		UseShortOptionHandling: true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Path to config file (default: ./gpcli.config)",
			},
		},
		Before: func(c *cli.Context) error {
			// Set config path from global flag before any command runs
			if configPath := c.String("config"); configPath != "" {
				src.ConfigPath = configPath
			}
			return nil
		},
		Commands: []*cli.Command{
			{
				Name:      "upload",
				Usage:     "Upload a file or directory to Google Photos",
				ArgsUsage: "[flags] <filepath>",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "recursive",
						Aliases: []string{"r"},
						Usage:   "Include subdirectories",
					},
					&cli.IntFlag{
						Name:    "threads",
						Aliases: []string{"t"},
						Value:   3,
						Usage:   "Number of upload threads",
					},
					&cli.BoolFlag{
						Name:    "force",
						Aliases: []string{"f"},
						Usage:   "Force upload even if file exists",
					},
					&cli.BoolFlag{
						Name:    "delete",
						Aliases: []string{"d"},
						Usage:   "Delete from host after upload",
					},
					&cli.BoolFlag{
						Name:    "disable-filter",
						Aliases: []string{"df"},
						Usage:   "Disable file type filtering",
					},
					&cli.StringFlag{
						Name:    "log-level",
						Aliases: []string{"l"},
						Value:   "info",
						Usage:   "Set log level: debug, info, warn, error",
					},
				},
				Action: uploadAction,
			},
			{
				Name:    "credentials",
				Aliases: []string{"creds"},
				Usage:   "Manage Google Photos credentials",
				Subcommands: []*cli.Command{
					{
						Name:      "add",
						Usage:     "Add a new credential",
						ArgsUsage: "<auth-string>",
						Action:    credentialsAddAction,
					},
					{
						Name:      "remove",
						Aliases:   []string{"rm"},
						Usage:     "Remove a credential by email",
						ArgsUsage: "<email>",
						Action:    credentialsRemoveAction,
					},
					{
						Name:    "list",
						Aliases: []string{"ls"},
						Usage:   "List all credentials",
						Action:  credentialsListAction,
					},
					{
						Name:      "set",
						Aliases:   []string{"select"},
						Usage:     "Set active credential (supports partial matching)",
						ArgsUsage: "<email>",
						Action:    credentialsSetAction,
					},
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func uploadAction(c *cli.Context) error {
	if c.NArg() < 1 {
		return fmt.Errorf("filepath required")
	}

	filePath := c.Args().First()

	// Validate that filepath exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file or directory does not exist: %s", filePath)
	}

	// Load backend config
	err := src.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override config with CLI flags
	threads := c.Int("threads")
	src.AppConfig.Recursive = c.Bool("recursive")
	src.AppConfig.UploadThreads = threads
	src.AppConfig.ForceUpload = c.Bool("force")
	src.AppConfig.DeleteFromHost = c.Bool("delete")
	src.AppConfig.DisableUnsupportedFilesFilter = c.Bool("disable-filter")

	// Parse log level
	logLevel := parseLogLevel(c.String("log-level"))

	// Track results
	var mu sync.Mutex
	var totalFiles int
	var completed int
	var failed int
	done := make(chan struct{})

	// Create CLI app with event callback
	eventCallback := func(event string, data any) {
		mu.Lock()
		defer mu.Unlock()

		switch event {
		case "uploadStart":
			if start, ok := data.(src.UploadBatchStart); ok {
				totalFiles = start.Total
				fmt.Printf("Uploading to Google Photos (%d threads)\n", threads)
				fmt.Printf("Found %d files to upload\n\n", totalFiles)
			}
		case "ThreadStatus":
			if status, ok := data.(src.ThreadStatus); ok {
				fmt.Printf("[%d] %s: %s\n", status.WorkerID, status.Status, status.FileName)
			}
		case "FileStatus":
			if result, ok := data.(src.FileUploadResult); ok {
				if result.IsError {
					failed++
					fmt.Printf("  FAILED: %s", result.Path)
					if result.Error != nil {
						fmt.Printf(" (%s)", result.Error.Error())
					}
					fmt.Println()
				} else {
					completed++
					fmt.Printf("  SUCCESS: %s\n", result.Path)
				}
			}
		case "uploadStop":
			close(done)
		}
	}

	cliApp := src.NewCLIApp(eventCallback, logLevel)
	uploadManager := src.NewUploadManager(cliApp)

	// Run upload in background
	go func() {
		uploadManager.Upload(cliApp, []string{filePath})
	}()

	// Wait for upload to complete
	<-done

	// Print summary
	fmt.Printf("\nUpload complete!\n")
	fmt.Printf("  Total: %d\n", totalFiles)
	fmt.Printf("  Succeeded: %d\n", completed)
	fmt.Printf("  Failed: %d\n", failed)

	return nil
}

func loadConfig() error {
	return src.LoadConfig()
}

func credentialsAddAction(c *cli.Context) error {
	if c.NArg() < 1 {
		return fmt.Errorf("auth-string required")
	}

	if err := loadConfig(); err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	authString := c.Args().First()
	configManager := &src.ConfigManager{}

	if err := configManager.AddCredentials(authString); err != nil {
		return fmt.Errorf("error adding credentials: %w", err)
	}

	fmt.Println("Credentials added successfully")
	return nil
}

func credentialsRemoveAction(c *cli.Context) error {
	if c.NArg() < 1 {
		return fmt.Errorf("email required")
	}

	if err := loadConfig(); err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	email := c.Args().First()
	configManager := &src.ConfigManager{}

	if err := configManager.RemoveCredentials(email); err != nil {
		return fmt.Errorf("error removing credentials: %w", err)
	}

	fmt.Printf("Credentials for %s removed successfully\n", email)
	return nil
}

func credentialsListAction(c *cli.Context) error {
	if err := loadConfig(); err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	configManager := &src.ConfigManager{}
	config := configManager.GetConfig()

	if len(config.Credentials) == 0 {
		fmt.Println("No credentials found")
		return nil
	}

	fmt.Println("Credentials:")
	for i, cred := range config.Credentials {
		params, err := src.ParseAuthString(cred)
		if err != nil {
			fmt.Printf("  %d. [Invalid credential]\n", i+1)
			continue
		}
		email := params.Get("Email")
		marker := " "
		if email == config.Selected {
			marker = "*"
		}
		fmt.Printf("  %s %s\n", marker, email)
	}

	if config.Selected != "" {
		fmt.Printf("\n* = active\n")
	}
	fmt.Printf("\nUse 'gpcli creds set <email>' to change active account (supports partial matching)\n")

	return nil
}

func credentialsSetAction(c *cli.Context) error {
	if c.NArg() < 1 {
		return fmt.Errorf("email required")
	}

	if err := loadConfig(); err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	query := c.Args().First()
	configManager := &src.ConfigManager{}
	config := configManager.GetConfig()

	// Try to find exact match first
	var matchedEmail string
	for _, cred := range config.Credentials {
		params, err := src.ParseAuthString(cred)
		if err != nil {
			continue
		}
		email := params.Get("Email")
		if email == query {
			matchedEmail = email
			break
		}
	}

	// If no exact match, try fuzzy matching (substring match)
	if matchedEmail == "" {
		var candidates []string
		for _, cred := range config.Credentials {
			params, err := src.ParseAuthString(cred)
			if err != nil {
				continue
			}
			email := params.Get("Email")
			if containsSubstring(email, query) {
				candidates = append(candidates, email)
			}
		}

		if len(candidates) == 0 {
			return fmt.Errorf("no credentials found matching '%s'", query)
		} else if len(candidates) == 1 {
			matchedEmail = candidates[0]
		} else {
			fmt.Fprintf(os.Stderr, "Error: multiple credentials match '%s':\n", query)
			for _, email := range candidates {
				fmt.Fprintf(os.Stderr, "  - %s\n", email)
			}
			return fmt.Errorf("please be more specific")
		}
	}

	configManager.SetSelected(matchedEmail)
	fmt.Printf("Active credential set to %s\n", matchedEmail)

	return nil
}

func containsSubstring(str, substr string) bool {
	strLower := strings.ToLower(str)
	substrLower := strings.ToLower(substr)
	return strings.Contains(strLower, substrLower)
}
