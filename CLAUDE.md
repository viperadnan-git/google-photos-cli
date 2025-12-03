# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

```bash
make build      # Build to ./gpcli
make clean      # Remove built binary
go build -o gpcli .   # Direct build command
```

## Architecture

This is a CLI tool for uploading photos/videos to Google Photos using an unofficial API. It uses protobuf for API communication.

### Key Components

- **cli.go** - CLI command definitions using urfave/cli/v2. Defines `upload` and `credentials` commands with their flags and actions.
- **src/** - Core business logic package
  - `api.go` - Google Photos API client. Handles authentication, upload tokens, file uploads, and commit operations via protobuf.
  - `upload.go` - Upload orchestration with worker pool for concurrent uploads. Emits events for progress tracking.
  - `configmanager.go` - YAML config file management using koanf. Stores credentials and settings.
  - `app.go` - GooglePhotosCLI struct that handles event emission and logging.
- **pb/** - Protobuf-generated Go code for API request/response structures.

### Event-Based Progress System

The upload system uses an event callback pattern:
1. `UploadManager.Upload()` starts worker goroutines
2. Workers emit events via `GooglePhotosCLI.EmitEvent()`
3. `cli.go` receives events and prints progress to stdout

Event types: `uploadStart`, `ThreadStatus`, `FileStatus`, `uploadStop`

### Config File

Config is stored in `./gpcli.config` (YAML) or custom path via `--config` flag. Contains credentials array and upload settings.
