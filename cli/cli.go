package cli

import (
	"context"
	"gpcli/gogpm"
	"log/slog"
	"os"
	"strings"

	"github.com/urfave/cli/v3"
)

// Run executes the CLI application
func Run() {
	cmd := &cli.Command{
		Name:                   "gpcli",
		Usage:                  "Google Photos unofficial CLI client",
		Version:                gogpm.Version,
		UseShortOptionHandling: true,
		EnableShellCompletion:  true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "config",
				Aliases:     []string{"c"},
				Usage:       "Path to config file",
				Sources:     cli.EnvVars("GPCLI_CONFIG"),
				DefaultText: "./gpcli.config",
			},
			&cli.StringFlag{
				Name:    "log-level",
				Value:   "info",
				Usage:   "Set log level: debug, info, warn, error",
				Sources: cli.EnvVars("GPCLI_LOG_LEVEL"),
			},
			&cli.BoolFlag{
				Name:    "quiet",
				Aliases: []string{"q"},
				Usage:   "Suppress all log output (overrides --log-level)",
				Sources: cli.EnvVars("GPCLI_QUIET"),
			},
			&cli.StringFlag{
				Name:    "auth",
				Usage:   "Authentication string (overrides config file)",
				Sources: cli.EnvVars("GPCLI_AUTH"),
			},
			&cli.StringFlag{
				Name:    "log-format",
				Value:   "human",
				Usage:   "Log format: human, slog, or json",
				Sources: cli.EnvVars("GPCLI_LOG_FORMAT"),
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
				Usage:     "Download a media item",
				ArgsUsage: "<media_key|dedup_key|file_path>",
				Flags: []cli.Flag{
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
				ArgsUsage: "<media_key|dedup_key|file_path>",
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
					&cli.BoolFlag{
						Name:  "jpeg",
						Usage: "Force JPEG format output",
					},
					&cli.BoolFlag{
						Name:  "overlay",
						Usage: "Show video overlay icon (hidden by default)",
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
			{
				Name:      "delete",
				Usage:     "Move item to trash or restore from trash",
				ArgsUsage: "<media_key|dedup_key|file_path>",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "restore",
						Usage:   "Restore from trash instead of delete",
						Aliases: []string{"r"},
					},
				},
				Action: deleteAction,
			},
			{
				Name:      "archive",
				Usage:     "Archive or unarchive item",
				ArgsUsage: "<media_key|dedup_key|file_path>",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "unarchive",
						Aliases: []string{"r", "u"},
						Usage:   "Unarchive instead of archive",
					},
				},
				Action: archiveAction,
			},
			{
				Name:      "favourite",
				Usage:     "Add or remove favourite status",
				ArgsUsage: "<media_key|dedup_key|file_path>",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "remove",
						Aliases: []string{"r"},
						Usage:   "Remove favourite status",
					},
				},
				Action: favouriteAction,
			},
			{
				Name:      "caption",
				Usage:     "Set item caption",
				ArgsUsage: "<media_key|dedup_key|file_path> <caption>",
				Action:    captionAction,
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		slog.Error("command failed", "error", err)
		os.Exit(1)
	}
}
