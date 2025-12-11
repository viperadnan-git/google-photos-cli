# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

```bash
make build      # Build to ./gpcli
make clean      # Remove built binary
cd cmd/gpcli && go build -o ../../gpcli .   # Direct build command
```

## Protobuf Generation

See `.proto/README.md` for detailed instructions on generating protobuf files.

Quick reference from project root:
```bash
export PATH=$PATH:$(go env GOPATH)/bin

# Generate single file
protoc --proto_path=. --go_out=. --go_opt=module=github.com/viperadnan-git/gogpm .proto/MessageName.proto

# Generate all files
for proto in .proto/*.proto; do
  protoc --proto_path=. --go_out=. --go_opt=module=github.com/viperadnan-git/gogpm "$proto"
done
```

## Architecture

This is a monorepo containing both a CLI tool and a Go library for managing Google Photos using an unofficial API. It uses protobuf for API communication.

### Project Structure

```
gogpm/
├── cmd/
│   └── gpcli/           # CLI application (separate module)
│       ├── go.mod       # module github.com/viperadnan-git/gogpm/cmd/gpcli
│       ├── main.go      # Entry point + command definitions
│       ├── config.go    # YAML config file management
│       └── ...
├── internal/
│   ├── core/            # Low-level API operations (internal)
│   │   ├── api.go       # Api struct with auth token management
│   │   ├── upload.go    # Upload token, file upload, commit
│   │   ├── download.go  # Download URL retrieval
│   │   └── ...
│   └── pb/              # Protobuf-generated Go code (internal)
├── .proto/              # Protobuf definitions
├── gogpm.go             # Public API: GooglePhotosAPI struct
├── uploader.go          # Upload orchestration with worker pool
├── sha1.go              # File hash calculation
├── utils.go             # Download utilities, key resolution
├── version.go           # Version constant
└── go.mod               # module github.com/viperadnan-git/gogpm (library only)
```

### Module Structure

The project uses two Go modules to separate library and CLI dependencies:

- **Root module** (`go.mod`): `github.com/viperadnan-git/gogpm` - Library with minimal dependencies (protobuf, retryablehttp)
- **CLI module** (`cmd/gpcli/go.mod`): `github.com/viperadnan-git/gogpm/cmd/gpcli` - CLI with additional dependencies (urfave/cli, koanf, go-selfupdate)

The CLI module uses a `replace` directive for local development:
```go
replace github.com/viperadnan-git/gogpm => ../..
```

### Key Components

- **Root package (gogpm)** - Public library API
  - `api.go` - GooglePhotosAPI struct embedding internal/core.Api
  - `uploader.go` - Upload orchestration with worker pool. Emits progress events.
  - `utils.go` - Download utilities, ResolveItemKey, ResolveMediaKey
  - `sha1.go` - File hash calculation
  - `version.go` - Version constant
- **cmd/gpcli/** - CLI application using urfave/cli/v3
  - `main.go` - Entry point + command definitions for upload, download, thumbnail, auth, delete, archive, favourite, caption, upgrade
  - `config.go` - YAML config file management. Stores credentials and settings.
- **internal/core/** - Low-level API operations (not exported)
  - `api.go` - Api struct with auth token management and common headers
  - `upload.go` - Upload token, file upload, commit operations
  - `download.go` - Download URL retrieval
  - `trash.go` - MoveToTrash, RestoreFromTrash operations
  - `archive.go` - SetArchived operation
  - `metadata.go` - SetCaption, SetFavourite operations
  - `album.go` - CreateAlbum, AddMediaToAlbum operations
  - `thumbnail.go` - Thumbnail download
  - `utils.go` - SHA1ToDedupeKey, ToURLSafeBase64
- **internal/pb/** - Protobuf-generated Go code for API request/response structures (not exported)
- **.proto/** - Protobuf definitions for Google Photos API messages

### Library Usage

```go
import "github.com/viperadnan-git/gogpm"

// Create API client
api, err := gogpm.NewGooglePhotosAPI(gogpm.ApiConfig{
    AuthData: authString,
})

// Upload files
api.Upload(paths, opts, callback)

// Download files
gogpm.DownloadFile(url, outputPath)
```

### Event-Based Progress System

The upload system uses an event callback pattern:
1. `GooglePhotosAPI.Upload()` starts worker goroutines
2. Workers emit events via callback function
3. CLI receives events and prints progress to stdout

Event types: `uploadStart`, `ThreadStatus`, `FileStatus`, `uploadStop`

### Config File

Config is stored in `./gpcli.config` (YAML) or custom path via `--config` flag. Contains credentials array and upload settings.

## Rules

- always implement root fixes and never add patch fixes
