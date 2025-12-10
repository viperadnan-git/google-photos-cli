package cli

import (
	"bytes"
	"context"
	"fmt"
	"gpcli/gogpm"
	"gpcli/gogpm/core"
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
var configPath string
var authOverride string
var cfgManager *ConfigManager

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

// Run executes the CLI application
func Run() {
	cmd := &cli.Command{
		Name:                   "gpcli",
		Usage:                  "Google Photos unofficial CLI client",
		Version:                gogpm.Version,
		UseShortOptionHandling: true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Path to config file (default: ./gpcli.config)",
			},
			&cli.StringFlag{
				Name:  "log-level",
				Value: "info",
				Usage: "Set log level: debug, info, warn, error",
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

			// Set config path from flag
			configPath = cmd.String("config")

			// Set auth override from flag (strip whitespace)
			if auth := cmd.String("auth"); auth != "" {
				authOverride = strings.TrimSpace(auth)
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
						Name:    "output",
						Aliases: []string{"o"},
						Usage:   "Output path (file path or directory)",
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

func loadConfig() error {
	var err error
	cfgManager, err = NewConfigManager(configPath)
	return err
}

// getAuthData returns the auth data string based on authOverride or selected config
func getAuthData(cfg Config) string {
	if authOverride != "" {
		return authOverride
	}
	// Find credentials for selected account
	for _, cred := range cfg.Credentials {
		params, err := ParseAuthString(cred)
		if err != nil {
			continue
		}
		if params.Get("Email") == cfg.Selected {
			return cred
		}
	}
	// Return first credential if no match
	if len(cfg.Credentials) > 0 {
		return cfg.Credentials[0]
	}
	return ""
}

func downloadAction(ctx context.Context, cmd *cli.Command) error {
	if err := loadConfig(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	cfg := cfgManager.GetConfig()

	mediaKey := cmd.Args().First()
	if mediaKey == "" {
		return fmt.Errorf("media_key is required")
	}

	getOriginal := cmd.Bool("original")
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

	if !urlOnly {
		logger.Info("fetching download URL", "media_key", mediaKey)
	}

	editedURL, originalURL, err := apiClient.GetDownloadUrls(mediaKey)
	if err != nil {
		return fmt.Errorf("failed to get download URLs: %w", err)
	}

	// Select the URL to use
	var downloadURL string
	var urlType string
	if getOriginal {
		if originalURL != "" {
			downloadURL = originalURL
			urlType = "original"
		} else {
			return fmt.Errorf("original URL not available")
		}
	} else {
		// Prefer edited URL, fallback to original
		if editedURL != "" {
			downloadURL = editedURL
			urlType = "edited"
		} else if originalURL != "" {
			downloadURL = originalURL
			urlType = "original"
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
	logger.Info("downloading", "url_type", urlType)
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

	mediaKey := cmd.Args().First()
	if mediaKey == "" {
		return fmt.Errorf("media_key is required")
	}

	outputPath := cmd.String("output")
	width := int(cmd.Int("width"))
	height := int(cmd.Int("height"))

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

	logger.Info("downloading thumbnail", "media_key", mediaKey)

	// Build thumbnail URL (force_jpeg=true, no_overlay=true by default)
	thumbnailURL := apiClient.GetThumbnailURL(mediaKey, width, height, true, true)

	// Download the thumbnail
	savedPath, err := gogpm.DownloadThumbnail(apiClient, thumbnailURL, outputPath, mediaKey)
	if err != nil {
		return err
	}
	logger.Info("thumbnail saved", "path", savedPath)
	return nil
}

func authInfoAction(ctx context.Context, cmd *cli.Command) error {
	// Check if --auth flag is set
	if authOverride != "" {
		params, err := ParseAuthString(authOverride)
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

	config := cfgManager.GetConfig()

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
		params, err := ParseAuthString(cred)
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

	if err := cfgManager.AddCredentials(authString); err != nil {
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
	config := cfgManager.GetConfig()

	email, err := resolveEmailFromArg(arg, config.Credentials)
	if err != nil {
		return err
	}

	if err := cfgManager.RemoveCredentials(email); err != nil {
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
	config := cfgManager.GetConfig()

	email, err := resolveEmailFromArg(arg, config.Credentials)
	if err != nil {
		return err
	}

	cfgManager.SetSelected(email)
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
		params, err := ParseAuthString(credentials[idx-1])
		if err != nil {
			return "", fmt.Errorf("invalid credential at index %d", idx)
		}
		return params.Get("Email"), nil
	}

	// Otherwise treat as email - try exact match first
	for _, cred := range credentials {
		params, err := ParseAuthString(cred)
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
		params, err := ParseAuthString(cred)
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
