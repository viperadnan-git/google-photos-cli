package main

import (
	"context"
	"log/slog"
	"os"
	"strings"

	gpm "github.com/viperadnan-git/go-gpm"

	"github.com/urfave/cli/v3"
)

func main() {
	cmd := &cli.Command{
		Name:                   "gpcli",
		Usage:                  "Google Photos unofficial CLI client",
		Version:                gpm.Version,
		UseShortOptionHandling: true,
		EnableShellCompletion:  true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "config",
				Aliases:     []string{"c"},
				Usage:       "Path to config file",
				Sources:     cli.EnvVars("GPCLI_CONFIG"),
				DefaultText: "~/.config/gpcli/gpcli.config",
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
				Name:  "upload",
				Usage: "Upload a file or directory to Google Photos",
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name:      "filepath",
						UsageText: "Path to the file or directory to upload",
					},
				},
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
						Name:  "disable-filter",
						Usage: "Disable file type filtering",
					},
					&cli.StringFlag{
						Name:  "album",
						Usage: "Add uploaded files to album with this name (creates if not exists)",
					},
					&cli.StringFlag{
						Name:    "quality",
						Aliases: []string{"q"},
						Value:   "original",
						Usage:   "Upload quality: 'original' or 'storage-saver'",
					},
					&cli.BoolFlag{
						Name:  "use-quota",
						Usage: "Uploaded files will count against your Google Photos storage quota",
					},
					&cli.BoolFlag{
						Name:    "archive",
						Aliases: []string{"a"},
						Usage:   "Archive uploaded files after upload",
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
				Name:  "download",
				Usage: "Download a media item",
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name:      "input",
						UsageText: "Media key, dedup key, or file path",
					},
				},
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
				Name:  "thumbnail",
				Usage: "Download thumbnail for a media item",
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name:      "input",
						UsageText: "Media key, dedup key, or file path",
					},
				},
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
						Name:  "add",
						Usage: "Add a new authentication",
						Arguments: []cli.Argument{
							&cli.StringArg{
								Name:      "auth-string",
								UsageText: "Authentication string from browser",
							},
						},
						Action: credentialsAddAction,
					},
					{
						Name:    "list",
						Aliases: []string{"ls"},
						Usage:   "List all authentications",
						Action:  authInfoAction,
					},
					{
						Name:    "remove",
						Aliases: []string{"rm"},
						Usage:   "Remove an authentication by number or email",
						Arguments: []cli.Argument{
							&cli.StringArg{
								Name:      "identifier",
								UsageText: "Account number or email address",
							},
						},
						Action: credentialsRemoveAction,
					},
					{
						Name:  "set",
						Usage: "Set active authentication by number or email",
						Arguments: []cli.Argument{
							&cli.StringArg{
								Name:      "identifier",
								UsageText: "Account number or email address",
							},
						},
						Action: credentialsSetAction,
					},
					{
						Name:   "file",
						Usage:  "Print config file path",
						Action: authFileAction,
					},
				},
			},
			{
				Name:      "delete",
				Usage:     "Move items to trash, restore from trash, or permanently delete",
				UsageText: "gpcli delete <input> [input...] [--from-file FILE]",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "restore",
						Usage:   "Restore from trash instead of delete",
						Aliases: []string{"r"},
					},
					&cli.BoolFlag{
						Name:    "force",
						Usage:   "Permanently delete (can't be undone)",
						Aliases: []string{"f"},
					},
					&cli.StringFlag{
						Name:    "from-file",
						Aliases: []string{"i"},
						Usage:   "Read item keys from file (one per line)",
					},
				},
				Action: deleteAction,
			},
			{
				Name:      "archive",
				Usage:     "Archive or unarchive items",
				UsageText: "gpcli archive <input> [input...] [--from-file FILE]",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "unarchive",
						Aliases: []string{"r", "u"},
						Usage:   "Unarchive instead of archive",
					},
					&cli.StringFlag{
						Name:    "from-file",
						Aliases: []string{"i"},
						Usage:   "Read item keys from file (one per line)",
					},
				},
				Action: archiveAction,
			},
			{
				Name:  "favourite",
				Usage: "Add or remove favourite status",
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name:      "input",
						UsageText: "Media key, dedup key, or file path",
					},
				},
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
				Name:  "caption",
				Usage: "Set item caption",
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name:      "input",
						UsageText: "Media key, dedup key, or file path",
					},
					&cli.StringArg{
						Name:      "caption",
						UsageText: "Caption text to set",
					},
				},
				Action: captionAction,
			},
			{
				Name:  "album",
				Usage: "Manage albums",
				Commands: []*cli.Command{
					{
						Name:      "create",
						Usage:     "Create a new album with media items",
						UsageText: "gpcli album create <name> <media-key> [media-key...]",
						Arguments: []cli.Argument{
							&cli.StringArg{
								Name:      "name",
								UsageText: "Album name",
							},
						},
						Action: albumCreateAction,
					},
					{
						Name:      "add",
						Usage:     "Add media items to an existing album",
						UsageText: "gpcli album add <album-key> <media-key> [media-key...] [--from-file FILE]",
						Arguments: []cli.Argument{
							&cli.StringArg{
								Name:      "album-key",
								UsageText: "Album media key",
							},
						},
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "from-file",
								Aliases: []string{"i"},
								Usage:   "Read media keys from file (one per line)",
							},
						},
						Action: albumAddAction,
					},
					{
						Name:      "rename",
						Aliases:   []string{"mv"},
						Usage:     "Rename an album",
						UsageText: "gpcli album rename <album-key> <new-name>",
						Arguments: []cli.Argument{
							&cli.StringArg{
								Name:      "album-key",
								UsageText: "Album media key",
							},
							&cli.StringArg{
								Name:      "new-name",
								UsageText: "New album name",
							},
						},
						Action: albumRenameAction,
					},
					{
						Name:      "delete",
						Aliases:   []string{"rm"},
						Usage:     "Delete an album",
						UsageText: "gpcli album delete <album-key>",
						Arguments: []cli.Argument{
							&cli.StringArg{
								Name:      "album-key",
								UsageText: "Album media key",
							},
						},
						Action: albumDeleteAction,
					},
				},
			},
			{
				Name:  "upgrade",
				Usage: "Upgrade gpcli to latest or specific version",
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name:      "version",
						UsageText: "Target version (optional, defaults to latest)",
					},
				},
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "check",
						Aliases: []string{"C"},
						Usage:   "Only check for updates without installing",
					},
				},
				Action: upgradeAction,
			},
			{
				Name:   "library-test",
				Usage:  "Test library state API (temporary command for development)",
				Hidden: true,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "state-token",
						Usage: "State token from previous library state call",
					},
					&cli.StringFlag{
						Name:  "page-token",
						Usage: "Page token for pagination",
					},
					&cli.BoolFlag{
						Name:  "raw",
						Usage: "Print raw hex dump instead of parsed JSON",
					},
					&cli.BoolFlag{
						Name:  "raw-json",
						Usage: "Print raw protobuf as JSON (for debugging structure)",
					},
				},
				Action: libraryTestAction,
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		slog.Error("command failed", "error", err)
		os.Exit(1)
	}
}
