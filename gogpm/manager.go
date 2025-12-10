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

	"gpcli/gogpm/core"
)

// ProgressCallback is a function type for upload progress updates
type ProgressCallback func(event string, data any)

// UploadOptions contains runtime options for upload operations
type UploadOptions struct {
	Threads         int
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

// Event types
type UploadBatchStart struct {
	Total int
}

type FileUploadResult struct {
	MediaKey   string
	DedupKey   string // URL-safe base64 encoded SHA1 hash for archive operations
	IsError    bool
	IsExisting bool
	Error      error
	Path       string
}

// UploadResult is the return type for uploadFileWithCallback
type UploadResult struct {
	MediaKey   string
	DedupKey   string // URL-safe base64 encoded SHA1 hash for archive operations
	IsExisting bool
	Error      error
}

type ThreadStatus struct {
	WorkerID int
	Status   string // "idle", "hashing", "checking", "uploading", "finalizing", "completed", "error"
	FilePath string
	FileName string
	Message  string
}

// Upload uploads files to Google Photos with progress callbacks
func (g *GooglePhotosAPI) Upload(paths []string, opts UploadOptions, callback ProgressCallback) {
	if g.running {
		return
	}

	g.running = true
	g.cancel = make(chan struct{})

	// Ensure callback is not nil
	if callback == nil {
		callback = func(event string, data any) {}
	}

	targetPaths, err := filterGooglePhotosFiles(paths, opts.Recursive, opts.DisableFilter)
	if err != nil {
		callback("FileStatus", FileUploadResult{
			IsError: true,
			Error:   err,
		})
		callback("uploadStop", nil)
		g.running = false
		return
	}

	if len(targetPaths) == 0 {
		callback("uploadStop", nil)
		g.running = false
		return
	}

	callback("uploadStart", UploadBatchStart{
		Total: len(targetPaths),
	})

	threads := opts.Threads
	if threads < 1 {
		threads = 1
	}

	// Don't start more threads than files to process
	numWorkers := min(threads, len(targetPaths))

	// Create a worker pool for concurrent uploads
	workChan := make(chan string, len(targetPaths))
	results := make(chan FileUploadResult, len(targetPaths))

	// Start workers using shared Api
	for i := range numWorkers {
		g.wg.Add(1)
		go startUploadWorker(i, workChan, results, g.cancel, &g.wg, g.Api, opts, callback)
	}

	// Send work to workers
	go func() {
	LOOP:
		for _, path := range targetPaths {
			select {
			case <-g.cancel:
				break LOOP
			case workChan <- path:
			}
		}
		close(workChan)
	}()

	// Wait for workers to finish and close results channel
	go func() {
		g.wg.Wait()
		close(results)
	}()

	// Process results, then emit uploadStop when done
	go func() {
		for result := range results {
			callback("FileStatus", result)
			if result.IsError {
				slog.Error("upload error", "error", result.Error)
			} else {
				slog.Info("upload success", "path", result.Path)
			}
		}
		// Only emit uploadStop after all results have been processed
		callback("uploadStop", nil)
		g.running = false
	}()
}

// isSupportedByGooglePhotos checks if a file extension is supported by Google Photos
func isSupportedByGooglePhotos(filename string) bool {
	// Convert to lowercase for case-insensitive comparison
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return false
	}

	// Remove the dot from the extension
	ext = ext[1:]

	// Supported photo formats
	photoFormats := []string{
		"avif", "bmp", "gif", "heic", "ico",
		"jpg", "jpeg", "png", "tiff", "webp",
		"cr2", "cr3", "nef", "arw", "orf",
		"raf", "rw2", "pef", "sr2", "dng",
	}

	// Supported video formats
	videoFormats := []string{
		"3gp", "3g2", "asf", "avi", "divx",
		"m2t", "m2ts", "m4v", "mkv", "mmv",
		"mod", "mov", "mp4", "mpg", "mpeg",
		"mts", "tod", "wmv", "ts",
	}

	// Check if extension is in either supported format
	return slices.Contains(photoFormats, ext) || slices.Contains(videoFormats, ext)
}

func scanDirectoryForFiles(path string, recursive bool) ([]string, error) {
	var files []string

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		fullPath := filepath.Join(path, entry.Name())
		if entry.IsDir() {
			if recursive {
				subFiles, err := scanDirectoryForFiles(fullPath, recursive)
				if err != nil {
					return nil, err
				}
				files = append(files, subFiles...)
			}
		} else {
			files = append(files, fullPath)
		}
	}

	return files, nil
}

// filterGooglePhotosFiles returns a list of files that are supported by Google Photos
func filterGooglePhotosFiles(paths []string, recursive, disableFilter bool) ([]string, error) {
	var supportedFiles []string

	for _, path := range paths {
		fileInfo, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("error accessing path %s: %v", path, err)
		}

		if fileInfo.IsDir() {
			files, err := scanDirectoryForFiles(path, recursive)
			if err != nil {
				return nil, fmt.Errorf("error scanning directory %s: %v", path, err)
			}

			for _, file := range files {
				if disableFilter {
					supportedFiles = append(supportedFiles, file)
				} else {
					if isSupportedByGooglePhotos(file) {
						supportedFiles = append(supportedFiles, file)
					}
				}

			}
		} else {
			if disableFilter {
				supportedFiles = append(supportedFiles, path)
			} else {
				if isSupportedByGooglePhotos(path) {
					supportedFiles = append(supportedFiles, path)
				}
			}

		}
	}

	return supportedFiles, nil
}

func uploadFileWithCallback(ctx context.Context, apiClient *core.Api, filePath string, workerID int, opts UploadOptions, callback ProgressCallback) UploadResult {
	fileName := filepath.Base(filePath)
	mediakey := ""

	// Stage 1: Hashing
	callback("ThreadStatus", ThreadStatus{
		WorkerID: workerID,
		Status:   "hashing",
		FilePath: filePath,
		FileName: fileName,
		Message:  "Hashing...",
	})

	sha1HashBytes, err := CalculateSHA1(ctx, filePath)
	if err != nil {
		return UploadResult{Error: fmt.Errorf("error calculating hash file: %w", err)}
	}

	sha1HashBase64 := base64.StdEncoding.EncodeToString([]byte(sha1HashBytes))
	dedupKey := core.SHA1ToDedupeKey(sha1HashBytes)

	// Stage 2: Checking if exists in library
	if !opts.ForceUpload {
		callback("ThreadStatus", ThreadStatus{
			WorkerID: workerID,
			Status:   "checking",
			FilePath: filePath,
			FileName: fileName,
			Message:  "Checking if file exists in library...",
		})

		mediakey, err = apiClient.FindRemoteMediaByHash(sha1HashBytes)
		if err != nil {
			slog.Error("error checking for remote matches", "error", err)
		}
		if len(mediakey) > 0 {
			callback("ThreadStatus", ThreadStatus{
				WorkerID: workerID,
				Status:   "completed",
				FilePath: filePath,
				FileName: fileName,
				Message:  "Already in library",
			})
			if opts.DeleteFromHost {
				if err := os.Remove(filePath); err != nil {
					slog.Warn("failed to delete file", "path", filePath, "error", err)
				} else {
					slog.Debug("deleted file", "path", filePath)
				}
			}
			return UploadResult{MediaKey: mediakey, DedupKey: dedupKey, IsExisting: true}
		}
	}

	file, err := os.Open(filePath)
	if err != nil {
		return UploadResult{Error: fmt.Errorf("error opening file: %w", err)}
	}
	fileInfo, err := file.Stat()
	file.Close()

	if err != nil {
		return UploadResult{Error: fmt.Errorf("error getting file info: %w", err)}
	}

	// Stage 3: Uploading
	callback("ThreadStatus", ThreadStatus{
		WorkerID: workerID,
		Status:   "uploading",
		FilePath: filePath,
		FileName: fileName,
		Message:  "Uploading...",
	})

	token, err := apiClient.GetUploadToken(sha1HashBase64, fileInfo.Size())
	if err != nil {
		return UploadResult{Error: fmt.Errorf("error uploading file: %w", err)}
	}

	commitToken, err := apiClient.UploadFile(ctx, filePath, token)
	if err != nil {
		return UploadResult{Error: fmt.Errorf("error uploading file: %w", err)}
	}

	// Stage 4: Finalizing
	callback("ThreadStatus", ThreadStatus{
		WorkerID: workerID,
		Status:   "finalizing",
		FilePath: filePath,
		FileName: fileName,
		Message:  "Committing upload...",
	})

	mediaKey, err := apiClient.CommitUpload(commitToken, fileInfo.Name(), sha1HashBytes, fileInfo.ModTime().Unix(), opts.Quality, opts.UseQuota)
	if err != nil {
		return UploadResult{Error: fmt.Errorf("error commiting file: %w", err)}
	}

	if len(mediaKey) == 0 {
		return UploadResult{Error: fmt.Errorf("media key not received")}
	}

	if opts.DeleteFromHost {
		if err := os.Remove(filePath); err != nil {
			slog.Warn("failed to delete file", "path", filePath, "error", err)
		} else {
			slog.Debug("deleted file", "path", filePath)
		}
	}

	return UploadResult{MediaKey: mediaKey, DedupKey: dedupKey}
}

func startUploadWorker(workerID int, workChan <-chan string, results chan<- FileUploadResult, cancel <-chan struct{}, wg *sync.WaitGroup, apiClient *core.Api, opts UploadOptions, callback ProgressCallback) {
	defer wg.Done()

	// Emit idle status initially
	callback("ThreadStatus", ThreadStatus{
		WorkerID: workerID,
		Status:   "idle",
		Message:  "Waiting for files...",
	})

	for path := range workChan {
		select {
		case <-cancel:
			callback("ThreadStatus", ThreadStatus{
				WorkerID: workerID,
				Status:   "idle",
				Message:  "Cancelled",
			})
			return // Stop if cancellation is requested
		default:
			ctx, cancelUpload := context.WithCancel(context.Background())
			go func() {
				<-cancel // If global cancel happens, cancel this upload
				cancelUpload()
			}()

			result := uploadFileWithCallback(ctx, apiClient, path, workerID, opts, callback)
			if result.Error != nil {
				results <- FileUploadResult{IsError: true, Error: result.Error, Path: path}
				callback("ThreadStatus", ThreadStatus{
					WorkerID: workerID,
					Status:   "error",
					FilePath: path,
					FileName: filepath.Base(path),
					Message:  fmt.Sprintf("Error: %v", result.Error),
				})
			} else {
				// Execute post-upload operations immediately after each successful upload
				postUploadOps(apiClient, result.DedupKey, path, workerID, opts, callback)

				results <- FileUploadResult{IsError: false, IsExisting: result.IsExisting, Path: path, MediaKey: result.MediaKey, DedupKey: result.DedupKey}
				callback("ThreadStatus", ThreadStatus{
					WorkerID: workerID,
					Status:   "completed",
					FilePath: path,
					FileName: filepath.Base(path),
					Message:  "Completed",
				})
			}
			cancelUpload()

			// Mark as idle after completing file
			callback("ThreadStatus", ThreadStatus{
				WorkerID: workerID,
				Status:   "idle",
				Message:  "Waiting for next file...",
			})
		}
	}

	// Final idle status when no more work
	callback("ThreadStatus", ThreadStatus{
		WorkerID: workerID,
		Status:   "idle",
		Message:  "Finished",
	})
}

// postUploadOps executes caption, favourite, and archive operations immediately after upload
func postUploadOps(apiClient *core.Api, dedupKey, filePath string, workerID int, opts UploadOptions, callback ProgressCallback) {
	fileName := filepath.Base(filePath)

	// Set caption if configured
	if opts.Caption != "" {
		callback("ThreadStatus", ThreadStatus{
			WorkerID: workerID,
			Status:   "finalizing",
			FilePath: filePath,
			FileName: fileName,
			Message:  "Setting caption...",
		})
		if err := apiClient.SetCaption(dedupKey, opts.Caption); err != nil {
			slog.Error("failed to set caption", "path", filePath, "error", err)
		}
	}

	// Set favourite if configured
	if opts.ShouldFavourite {
		callback("ThreadStatus", ThreadStatus{
			WorkerID: workerID,
			Status:   "finalizing",
			FilePath: filePath,
			FileName: fileName,
			Message:  "Setting favourite...",
		})
		if err := apiClient.SetFavourite(dedupKey, true); err != nil {
			slog.Error("failed to set favourite", "path", filePath, "error", err)
		}
	}

	// Archive if configured
	if opts.ShouldArchive {
		callback("ThreadStatus", ThreadStatus{
			WorkerID: workerID,
			Status:   "finalizing",
			FilePath: filePath,
			FileName: fileName,
			Message:  "Archiving...",
		})
		if err := apiClient.SetArchived([]string{dedupKey}, true); err != nil {
			slog.Error("failed to archive", "path", filePath, "error", err)
		}
	}
}
