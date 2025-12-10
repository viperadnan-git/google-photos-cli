package main

import (
	"bytes"
	"context"
	"fmt"
	"gpcli/src"
	"gpcli/src/api"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/urfave/cli/v3"
)

var logger *slog.Logger
var currentLogLevel slog.Level
var logFormat string

// humanHandler is a slog.Handler that outputs human-readable logs without timestamps
type humanHandler struct {
	out   io.Writer
	level slog.Level
}

func (h *humanHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *humanHandler) Handle(_ context.Context, r slog.Record) error {
	var buf bytes.Buffer
	buf.WriteString(r.Message)
	r.Attrs(func(a slog.Attr) bool {
		buf.WriteString(" ")
		buf.WriteString(a.Key)
		buf.WriteString("=")
		buf.WriteString(fmt.Sprintf("%v", a.Value.Any()))
		return true
	})
	buf.WriteString("\n")
	_, err := h.out.Write(buf.Bytes())
	return err
}

func (h *humanHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *humanHandler) WithGroup(name string) slog.Handler       { return h }

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

// initLogger initializes the global logger with the specified level and format
func initLogger(level slog.Level) {
	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	switch logFormat {
	case "slog":
		handler = slog.NewTextHandler(os.Stdout, opts)
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default: // "human"
		handler = &humanHandler{out: os.Stdout, level: level}
	}
	logger = slog.New(handler)
	slog.SetDefault(logger)
}

// initQuietLogger initializes a logger that only shows errors
func initQuietLogger() {
	currentLogLevel = slog.LevelError
	initLogger(slog.LevelError)
}

func runCLI() {
	cmd := &cli.Command{
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
			&cli.StringFlag{
				Name:    "log-level",
				Value:   "info",
				Usage:   "Set log level: debug, info, warn, error",
			},
			&cli.BoolFlag{
				Name:    "quiet",
				Aliases: []string{"q"},
				Usage:   "Suppress all log output (overrides --log-level)",
			},
			&cli.StringFlag{
				Name:  "auth",
				Usage: "Authentication string (overrides config file)",
			},
			&cli.StringFlag{
				Name:  "log-format",
				Value: "human",
				Usage: "Log format: human (default), slog (machine-readable text), or json",
			},
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			// Set log format before initializing logger
			logFormat = cmd.String("log-format")

			// Initialize logger - quiet mode overrides log level
			if cmd.Bool("quiet") {
				initQuietLogger()
			} else {
				currentLogLevel = parseLogLevel(cmd.String("log-level"))
				initLogger(currentLogLevel)
			}

			// Set config path from global flag before any command runs
			if configPath := cmd.String("config"); configPath != "" {
				src.ConfigPath = configPath
			}

			// Set auth override from flag (strip whitespace)
			if auth := cmd.String("auth"); auth != "" {
				src.AuthOverride = strings.TrimSpace(auth)
			}
			return ctx, nil
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
						Name:    "album",
						Aliases: []string{"a"},
						Usage:   "Add uploaded files to album with this name (creates if not exists)",
					},
					&cli.StringFlag{
						Name:    "quality",
						Aliases: []string{"Q"},
						Value:   "original",
						Usage:   "Upload quality: 'original' or 'storage-saver'",
					},
					&cli.BoolFlag{
						Name:  "use-quota",
						Usage: "Uploaded files will count against your Google Photos storage quota",
					},
					&cli.BoolFlag{
						Name:  "archive",
						Usage: "Archive uploaded files after upload",
					},
					&cli.StringFlag{
						Name:  "caption",
						Usage: "Set caption for uploaded files",
					},
					&cli.BoolFlag{
						Name:  "favourite",
						Usage: "Mark uploaded files as favourites",
					},
				},
				Action: uploadAction,
			},
			{
				Name:      "download",
				Usage:     "Download a media item by media key",
				ArgsUsage: "<media_key>",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "original",
						Usage: "Download original file (default: edited if available)",
					},
					&cli.BoolFlag{
						Name:  "url",
						Usage: "Only print download URL without downloading",
					},
					&cli.StringFlag{
						Name:  "output",
						Aliases: []string{"o"},
						Usage: "Output path (file path or directory)",
					},
				},
				Action: downloadAction,
			},
			{
				Name:      "thumbnail",
				Usage:     "Download thumbnail for a media item",
				ArgsUsage: "<media_key>",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "output",
						Aliases: []string{"o"},
						Usage:   "Output path (file path or directory)",
					},
					&cli.IntFlag{
						Name:    "width",
						Aliases: []string{"w"},
						Usage:   "Thumbnail width in pixels",
					},
					&cli.IntFlag{
						Name:  "height",
						Usage: "Thumbnail height in pixels",
					},
				},
				Action: thumbnailAction,
			},
			{
				Name:   "auth",
				Usage:  "Manage Google Photos authentication",
				Action: authInfoAction,
				Commands: []*cli.Command{
					{
						Name:      "add",
						Usage:     "Add a new authentication",
						ArgsUsage: "<auth-string>",
						Action:    credentialsAddAction,
					},
					{
						Name:    "list",
						Aliases: []string{"ls"},
						Usage:   "List all authentications",
						Action:  authInfoAction,
					},
					{
						Name:      "remove",
						Aliases:   []string{"rm"},
						Usage:     "Remove an authentication by number or email",
						ArgsUsage: "<number|email>",
						Action:    credentialsRemoveAction,
					},
					{
						Name:      "set",
						Usage:     "Set active authentication by number or email",
						ArgsUsage: "<number|email>",
						Action:    credentialsSetAction,
					},
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		slog.Error("command failed", "error", err)
		os.Exit(1)
	}
}

func uploadAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.NArg() < 1 {
		return fmt.Errorf("filepath required")
	}

	filePath := cmd.Args().First()

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
	threads := int(cmd.Int("threads"))
	src.AppConfig.Recursive = cmd.Bool("recursive")
	src.AppConfig.UploadThreads = threads
	src.AppConfig.ForceUpload = cmd.Bool("force")
	src.AppConfig.DeleteFromHost = cmd.Bool("delete")
	src.AppConfig.DisableUnsupportedFilesFilter = cmd.Bool("disable-filter")
	src.AppConfig.UseQuota = cmd.Bool("use-quota")

	// Handle quality flag
	quality := cmd.String("quality")
	if quality != "original" && quality != "storage-saver" {
		return fmt.Errorf("invalid quality: %s (use 'original' or 'storage-saver')", quality)
	}
	src.AppConfig.Quality = quality

	// Get album name and set runtime config for post-upload operations
	albumName := cmd.String("album")
	src.AppConfig.ShouldArchive = cmd.Bool("archive")
	src.AppConfig.Caption = cmd.String("caption")
	src.AppConfig.ShouldFavourite = cmd.Bool("favourite")

	// Log configuration at start
	logger.Info("starting upload",
		"path", filePath,
		"threads", threads,
		"recursive", src.AppConfig.Recursive,
		"force", src.AppConfig.ForceUpload,
		"delete", src.AppConfig.DeleteFromHost,
		"disable-filter", src.AppConfig.DisableUnsupportedFilesFilter,
		"quality", quality,
		"use-quota", src.AppConfig.UseQuota,
		"album", albumName,
		"archive", src.AppConfig.ShouldArchive,
		"caption", src.AppConfig.Caption,
		"favourite", src.AppConfig.ShouldFavourite,
	)

	// Track results
	var mu sync.Mutex
	var totalFiles int
	var uploaded int
	var existing int
	var failed int
	var successfulMediaKeys []string
	done := make(chan struct{})

	// Create CLI app with event callback
	eventCallback := func(event string, data any) {
		mu.Lock()
		defer mu.Unlock()

		switch event {
		case "uploadStart":
			if start, ok := data.(src.UploadBatchStart); ok {
				totalFiles = start.Total
				logger.Info("upload batch started", "total", totalFiles)
			}
		case "ThreadStatus":
			if status, ok := data.(src.ThreadStatus); ok {
				logger.Debug("worker status",
					"worker_id", status.WorkerID,
					"status", status.Status,
					"file", status.FileName,
				)
			}
		case "FileStatus":
			if result, ok := data.(src.FileUploadResult); ok {
				if result.IsError {
					failed++
					logger.Error("upload failed",
						"path", result.Path,
						"error", result.Error,
					)
				} else if result.IsExisting {
					existing++
					logger.Info("already exists",
						"path", result.Path,
						"media_key", result.MediaKey,
					)
					// Collect media key for album
					if result.MediaKey != "" {
						successfulMediaKeys = append(successfulMediaKeys, result.MediaKey)
					}
				} else {
					uploaded++
					logger.Info("upload success",
						"path", result.Path,
						"media_key", result.MediaKey,
					)
					// Collect media key for album
					if result.MediaKey != "" {
						successfulMediaKeys = append(successfulMediaKeys, result.MediaKey)
					}
				}
			}
		case "uploadStop":
			close(done)
		}
	}

	app := src.NewGooglePhotosCLI(eventCallback, currentLogLevel)
	uploadManager := src.NewUploadManager(app)

	// Run upload in background
	go func() {
		uploadManager.Upload(app, []string{filePath})
	}()

	// Wait for upload to complete
	<-done

	// Print summary
	logger.Info("upload complete",
		"total", totalFiles,
		"succeeded", uploaded+existing,
		"failed", failed,
		"uploaded", uploaded,
		"existing", existing,
	)

	// Handle album creation if album name was specified
	if albumName != "" && len(successfulMediaKeys) > 0 {
		logger.Info("creating album", "name", albumName, "items", len(successfulMediaKeys))

		apiClient, err := api.NewApi(api.ApiConfig{
			AuthOverride: src.AuthOverride,
			Selected:     src.AppConfig.Selected,
			Credentials:  src.AppConfig.Credentials,
			Proxy:        src.AppConfig.Proxy,
			Quality:      src.AppConfig.Quality,
			UseQuota:     src.AppConfig.UseQuota,
		})
		if err != nil {
			logger.Error("failed to create API client for album creation", "error", err)
			return fmt.Errorf("failed to create API client: %w", err)
		}

		// Create album with media keys (API may have batch limits, handle large sets)
		const batchSize = 500
		var albumMediaKey string

		if len(successfulMediaKeys) <= batchSize {
			// Create album with all media keys
			albumMediaKey, err = apiClient.CreateAlbum(albumName, successfulMediaKeys)
			if err != nil {
				logger.Error("failed to create album", "error", err)
				return fmt.Errorf("failed to create album: %w", err)
			}
		} else {
			// Create album with first batch
			albumMediaKey, err = apiClient.CreateAlbum(albumName, successfulMediaKeys[:batchSize])
			if err != nil {
				logger.Error("failed to create album", "error", err)
				return fmt.Errorf("failed to create album: %w", err)
			}

			// Add remaining items in batches
			for i := batchSize; i < len(successfulMediaKeys); i += batchSize {
				end := i + batchSize
				if end > len(successfulMediaKeys) {
					end = len(successfulMediaKeys)
				}
				err = apiClient.AddMediaToAlbum(albumMediaKey, successfulMediaKeys[i:end])
				if err != nil {
					logger.Error("failed to add items to album", "batch_start", i, "error", err)
				}
			}
		}

		logger.Info("album created", "name", albumName, "album_key", albumMediaKey, "items", len(successfulMediaKeys))
	}

	// Note: Caption, favourite, and archive operations are now executed immediately
	// after each file upload in the upload worker (src/upload.go postUploadOps)

	return nil
}

func loadConfig() error {
	return src.LoadConfig()
}

func downloadAction(ctx context.Context, cmd *cli.Command) error {
	if err := loadConfig(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	mediaKey := cmd.Args().First()
	if mediaKey == "" {
		return fmt.Errorf("media_key is required")
	}

	getOriginal := cmd.Bool("original")
	urlOnly := cmd.Bool("url")
	outputPath := cmd.String("output")

	apiClient, err := api.NewApi(api.ApiConfig{
		AuthOverride: src.AuthOverride,
		Selected:     src.AppConfig.Selected,
		Credentials:  src.AppConfig.Credentials,
		Proxy:        src.AppConfig.Proxy,
		Quality:      src.AppConfig.Quality,
		UseQuota:     src.AppConfig.UseQuota,
	})
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	editedURL, originalURL, err := apiClient.GetDownloadUrls(mediaKey)
	if err != nil {
		return fmt.Errorf("failed to get download URLs: %w", err)
	}

	// Select the URL to use
	var downloadURL string
	if getOriginal {
		if originalURL != "" {
			downloadURL = originalURL
		} else {
			return fmt.Errorf("original URL not available")
		}
	} else {
		// Prefer edited URL, fallback to original
		if editedURL != "" {
			downloadURL = editedURL
		} else if originalURL != "" {
			downloadURL = originalURL
		} else {
			return fmt.Errorf("no download URL available")
		}
	}

	// If --url flag is set, just print the URL and exit
	if urlOnly {
		fmt.Println(downloadURL)
		return nil
	}

	// Download the file
	return downloadFile(apiClient, downloadURL, outputPath)
}

func thumbnailAction(ctx context.Context, cmd *cli.Command) error {
	if err := loadConfig(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	mediaKey := cmd.Args().First()
	if mediaKey == "" {
		return fmt.Errorf("media_key is required")
	}

	outputPath := cmd.String("output")
	width := int(cmd.Int("width"))
	height := int(cmd.Int("height"))

	apiClient, err := api.NewApi(api.ApiConfig{
		AuthOverride: src.AuthOverride,
		Selected:     src.AppConfig.Selected,
		Credentials:  src.AppConfig.Credentials,
		Proxy:        src.AppConfig.Proxy,
	})
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	// Build thumbnail URL (force_jpeg=true, no_overlay=true by default)
	thumbnailURL := apiClient.GetThumbnailURL(mediaKey, width, height, true, true)

	// Download the thumbnail
	return downloadThumbnail(apiClient, thumbnailURL, outputPath, mediaKey)
}

func authInfoAction(ctx context.Context, cmd *cli.Command) error {
	// Check if --auth flag is set
	if src.AuthOverride != "" {
		params, err := src.ParseAuthString(src.AuthOverride)
		if err != nil {
			return fmt.Errorf("invalid auth string: %w", err)
		}
		fmt.Println("Current authentication (from --auth flag):")
		fmt.Printf("  Email: %s\n", params.Get("Email"))
		return nil
	}

	// Load from config
	if err := loadConfig(); err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	configManager := &src.ConfigManager{}
	config := configManager.GetConfig()

	// Show current authentication
	if config.Selected != "" {
		fmt.Printf("Current authentication: %s\n", config.Selected)
	} else {
		fmt.Println("No active authentication")
	}

	// List all available accounts
	if len(config.Credentials) == 0 {
		fmt.Println("\nNo accounts configured. Use 'gpcli auth add <auth-string>' to add one.")
		return nil
	}

	fmt.Println("\nAvailable accounts:")
	for i, cred := range config.Credentials {
		params, err := src.ParseAuthString(cred)
		if err != nil {
			fmt.Printf("  %d. [Invalid]\n", i+1)
			continue
		}
		email := params.Get("Email")
		marker := ""
		if email == config.Selected {
			marker = " *"
		}
		fmt.Printf("  %d. %s%s\n", i+1, email, marker)
	}

	fmt.Println("\nUse 'gpcli auth set <number|email>' to change active authentication")

	return nil
}

func credentialsAddAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.NArg() < 1 {
		return fmt.Errorf("auth-string required")
	}

	if err := loadConfig(); err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	authString := strings.TrimSpace(cmd.Args().First())
	configManager := &src.ConfigManager{}

	if err := configManager.AddCredentials(authString); err != nil {
		return fmt.Errorf("invalid credentials: %w", err)
	}

	slog.Info("authentication added successfully")
	return nil
}

func credentialsRemoveAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.NArg() < 1 {
		return fmt.Errorf("number or email required")
	}

	if err := loadConfig(); err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	arg := cmd.Args().First()
	configManager := &src.ConfigManager{}
	config := configManager.GetConfig()

	email, err := resolveEmailFromArg(arg, config.Credentials)
	if err != nil {
		return err
	}

	if err := configManager.RemoveCredentials(email); err != nil {
		return fmt.Errorf("error removing authentication: %w", err)
	}

	slog.Info("authentication removed", "email", email)
	return nil
}

func credentialsSetAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.NArg() < 1 {
		return fmt.Errorf("number or email required")
	}

	if err := loadConfig(); err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	arg := cmd.Args().First()
	configManager := &src.ConfigManager{}
	config := configManager.GetConfig()

	email, err := resolveEmailFromArg(arg, config.Credentials)
	if err != nil {
		return err
	}

	configManager.SetSelected(email)
	slog.Info("active account set", "email", email)

	return nil
}

func containsSubstring(str, substr string) bool {
	strLower := strings.ToLower(str)
	substrLower := strings.ToLower(substr)
	return strings.Contains(strLower, substrLower)
}

// resolveEmailFromArg resolves an email from either an index number (1-based) or email string
func resolveEmailFromArg(arg string, credentials []string) (string, error) {
	// Try to parse as number first
	if num, err := fmt.Sscanf(arg, "%d", new(int)); err == nil && num == 1 {
		var idx int
		fmt.Sscanf(arg, "%d", &idx)
		if idx < 1 || idx > len(credentials) {
			return "", fmt.Errorf("invalid index %d: must be between 1 and %d", idx, len(credentials))
		}
		params, err := src.ParseAuthString(credentials[idx-1])
		if err != nil {
			return "", fmt.Errorf("invalid credential at index %d", idx)
		}
		return params.Get("Email"), nil
	}

	// Otherwise treat as email - try exact match first
	for _, cred := range credentials {
		params, err := src.ParseAuthString(cred)
		if err != nil {
			continue
		}
		email := params.Get("Email")
		if email == arg {
			return email, nil
		}
	}

	// Try fuzzy matching
	var candidates []string
	for _, cred := range credentials {
		params, err := src.ParseAuthString(cred)
		if err != nil {
			continue
		}
		email := params.Get("Email")
		if containsSubstring(email, arg) {
			candidates = append(candidates, email)
		}
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no authentication found matching '%s'", arg)
	} else if len(candidates) == 1 {
		return candidates[0], nil
	}
	return "", fmt.Errorf("multiple accounts match '%s': %v - please be more specific", arg, candidates)
}
