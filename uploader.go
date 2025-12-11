package gogpm

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/viperadnan-git/gogpm/internal/core"
)

// UploadStatus represents the state of a file upload
type UploadStatus string

const (
	StatusHashing    UploadStatus = "hashing"
	StatusChecking   UploadStatus = "checking"
	StatusUploading  UploadStatus = "uploading"
	StatusFinalizing UploadStatus = "finalizing"
	StatusCompleted  UploadStatus = "completed"
	StatusSkipped    UploadStatus = "skipped" // Already in library
	StatusFailed     UploadStatus = "failed"
)

// UploadEvent represents a status update for a file upload
type UploadEvent struct {
	Path     string
	Status   UploadStatus
	MediaKey string
	DedupKey string
	Error    error
	WorkerID int
	Total    int // Total files in batch (set on first event)
}

// UploadOptions contains runtime options for upload operations
type UploadOptions struct {
	Workers         int
	Recursive       bool
	ForceUpload     bool
	DeleteFromHost  bool
	DisableFilter   bool
	Caption         string
	ShouldFavourite bool
	ShouldArchive   bool
	Quality         string // "original" or "storage-saver"
	UseQuota        bool
}

// Upload uploads files to Google Photos and returns a channel for status events.
// The channel is closed when upload completes. Multiple calls are queued automatically.
func (g *GooglePhotosAPI) Upload(ctx context.Context, paths []string, opts UploadOptions) <-chan UploadEvent {
	events := make(chan UploadEvent)

	go func() {
		// Serialize upload batches
		g.uploadMu.Lock()
		defer g.uploadMu.Unlock()
		defer close(events)

		// Filter files
		files, err := filterGooglePhotosFiles(paths, opts.Recursive, opts.DisableFilter)
		if err != nil {
			events <- UploadEvent{Status: StatusFailed, Error: err}
			return
		}
		if len(files) == 0 {
			return
		}

		// Send total count with first event
		workers := max(1, opts.Workers)
		workers = min(workers, len(files))

		workChan := make(chan string, len(files))
		var wg sync.WaitGroup

		// Start workers
		for i := range workers {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for path := range workChan {
					select {
					case <-ctx.Done():
						return
					default:
					}
					uploadFile(ctx, g.Api, path, workerID, opts, events)
				}
			}(i)
		}

		// Send work (with total on first)
		first := true
		for _, path := range files {
			select {
			case <-ctx.Done():
				close(workChan)
				wg.Wait()
				return
			default:
			}
			if first {
				events <- UploadEvent{Total: len(files)}
				first = false
			}
			workChan <- path
		}
		close(workChan)
		wg.Wait()
	}()

	return events
}

func uploadFile(ctx context.Context, api *core.Api, filePath string, workerID int, opts UploadOptions, events chan<- UploadEvent) {
	send := func(status UploadStatus, mediaKey, dedupKey string, err error) {
		events <- UploadEvent{
			Path: filePath, Status: status, MediaKey: mediaKey, DedupKey: dedupKey, Error: err, WorkerID: workerID,
		}
	}

	// Hash file
	send(StatusHashing, "", "", nil)
	sha1Hash, err := CalculateSHA1(ctx, filePath)
	if err != nil {
		send(StatusFailed, "", "", fmt.Errorf("hash error: %w", err))
		return
	}
	dedupKey := core.SHA1ToDedupeKey(sha1Hash)

	// Check if exists
	if !opts.ForceUpload {
		send(StatusChecking, "", dedupKey, nil)
		if mediaKey, _ := api.FindRemoteMediaByHash(sha1Hash); mediaKey != "" {
			if opts.DeleteFromHost {
				os.Remove(filePath)
			}
			send(StatusSkipped, mediaKey, dedupKey, nil)
			return
		}
	}

	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		send(StatusFailed, "", dedupKey, fmt.Errorf("stat error: %w", err))
		return
	}

	// Upload
	send(StatusUploading, "", dedupKey, nil)
	sha1Base64 := base64.StdEncoding.EncodeToString([]byte(sha1Hash))
	token, err := api.GetUploadToken(sha1Base64, fileInfo.Size())
	if err != nil {
		send(StatusFailed, "", dedupKey, fmt.Errorf("upload token error: %w", err))
		return
	}

	commitToken, err := api.UploadFile(ctx, filePath, token)
	if err != nil {
		send(StatusFailed, "", dedupKey, fmt.Errorf("upload error: %w", err))
		return
	}

	// Finalize
	send(StatusFinalizing, "", dedupKey, nil)
	mediaKey, err := api.CommitUpload(commitToken, fileInfo.Name(), sha1Hash, fileInfo.ModTime().Unix(), opts.Quality, opts.UseQuota)
	if err != nil {
		send(StatusFailed, "", dedupKey, fmt.Errorf("commit error: %w", err))
		return
	}
	if mediaKey == "" {
		send(StatusFailed, "", dedupKey, fmt.Errorf("no media key returned"))
		return
	}

	// Post-upload ops
	if opts.Caption != "" {
		if err := api.SetCaption(mediaKey, opts.Caption); err != nil {
			slog.Error("caption failed", "path", filePath, "error", err)
		}
	}
	if opts.ShouldFavourite {
		if err := api.SetFavourite(mediaKey, true); err != nil {
			slog.Error("favourite failed", "path", filePath, "error", err)
		}
	}
	if opts.ShouldArchive {
		if err := api.SetArchived([]string{mediaKey}, true); err != nil {
			slog.Error("archive failed", "path", filePath, "error", err)
		}
	}
	if opts.DeleteFromHost {
		os.Remove(filePath)
	}

	send(StatusCompleted, mediaKey, dedupKey, nil)
}

// isSupportedByGooglePhotos checks if a file extension is supported
func isSupportedByGooglePhotos(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return false
	}
	ext = ext[1:]

	photoFormats := []string{
		"avif", "bmp", "gif", "heic", "ico", "jpg", "jpeg", "png", "tiff", "webp",
		"cr2", "cr3", "nef", "arw", "orf", "raf", "rw2", "pef", "sr2", "dng",
	}
	videoFormats := []string{
		"3gp", "3g2", "asf", "avi", "divx", "m2t", "m2ts", "m4v", "mkv", "mmv",
		"mod", "mov", "mp4", "mpg", "mpeg", "mts", "tod", "wmv", "ts",
	}
	return slices.Contains(photoFormats, ext) || slices.Contains(videoFormats, ext)
}

func filterGooglePhotosFiles(paths []string, recursive, disableFilter bool) ([]string, error) {
	var result []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("error accessing %s: %w", path, err)
		}
		if info.IsDir() {
			files, err := scanDir(path, recursive)
			if err != nil {
				return nil, err
			}
			for _, f := range files {
				if disableFilter || isSupportedByGooglePhotos(f) {
					result = append(result, f)
				}
			}
		} else if disableFilter || isSupportedByGooglePhotos(path) {
			result = append(result, path)
		}
	}
	return result, nil
}

func scanDir(path string, recursive bool) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		full := filepath.Join(path, e.Name())
		if e.IsDir() && recursive {
			sub, err := scanDir(full, true)
			if err != nil {
				return nil, err
			}
			files = append(files, sub...)
		} else if !e.IsDir() {
			files = append(files, full)
		}
	}
	return files, nil
}
