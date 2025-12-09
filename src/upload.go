package src

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

	"gpcli/src/api"
)

// ProgressCallback is a function type for upload progress updates
type ProgressCallback func(event string, data any)

type UploadManager struct {
	wg      sync.WaitGroup
	cancel  chan struct{}
	running bool
	app     *GooglePhotosCLI
}

func NewUploadManager(app *GooglePhotosCLI) *UploadManager {
	return &UploadManager{
		app: app,
	}
}

func (m *UploadManager) IsRunning() bool {
	return m.running
}

func (m *UploadManager) Cancel() {
	if m.cancel != nil {
		close(m.cancel)
		m.cancel = nil
	}
}

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

func (m *UploadManager) Upload(app *GooglePhotosCLI, paths []string) {
	if m.running {
		return
	}

	m.running = true
	m.cancel = make(chan struct{})

	targetPaths, err := filterGooglePhotosFiles(paths)
	if err != nil {
		app.EmitEvent("FileStatus", FileUploadResult{
			IsError: true,
			Error:   err,
		})
		app.EmitEvent("uploadStop", nil)
		m.running = false
		return
	}

	if len(targetPaths) == 0 {
		app.EmitEvent("uploadStop", nil)
		m.running = false
		return
	}

	app.EmitEvent("uploadStart", UploadBatchStart{
		Total: len(targetPaths),
	})

	if AppConfig.UploadThreads < 1 {
		AppConfig.UploadThreads = 1
	}

	// Don't start more threads than files to process
	numWorkers := min(AppConfig.UploadThreads, len(targetPaths))

	// Create a worker pool for concurrent uploads
	workChan := make(chan string, len(targetPaths))
	results := make(chan FileUploadResult, len(targetPaths))

	// Start workers
	for i := range numWorkers {
		m.wg.Add(1)
		go startUploadWorker(i, workChan, results, m.cancel, &m.wg, app)
	}

	// Send work to workers
	go func() {
	LOOP:
		for _, path := range targetPaths {
			select {
			case <-m.cancel:
				break LOOP
			case workChan <- path:
			}
		}
		close(workChan)
	}()

	// Wait for workers to finish and close results channel
	go func() {
		m.wg.Wait()
		close(results)
	}()

	// Process results, then emit uploadStop when done
	go func() {
		for result := range results {
			app.EmitEvent("FileStatus", result)
			if result.IsError {
				s := fmt.Sprintf("upload error: %v", result.Error)
				app.GetLogger().Error(s)
			} else {
				s := fmt.Sprintf("upload success: %v", result.Path)
				app.GetLogger().Info(s)
			}
		}
		// Only emit uploadStop after all results have been processed
		app.EmitEvent("uploadStop", nil)
		m.running = false
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
func filterGooglePhotosFiles(paths []string) ([]string, error) {
	var supportedFiles []string

	for _, path := range paths {
		fileInfo, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("error accessing path %s: %v", path, err)
		}

		if fileInfo.IsDir() {
			files, err := scanDirectoryForFiles(path, AppConfig.Recursive)
			if err != nil {
				return nil, fmt.Errorf("error scanning directory %s: %v", path, err)
			}

			for _, file := range files {
				if AppConfig.DisableUnsupportedFilesFilter {
					supportedFiles = append(supportedFiles, file)
				} else {
					if isSupportedByGooglePhotos(file) {
						supportedFiles = append(supportedFiles, file)
					}
				}

			}
		} else {
			if AppConfig.DisableUnsupportedFilesFilter {
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

func uploadFileWithCallback(ctx context.Context, apiClient *api.Api, filePath string, workerID int, callback ProgressCallback) UploadResult {
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
	dedupKey := api.SHA1ToDedupeKey(sha1HashBytes)

	// Stage 2: Checking if exists in library
	if !AppConfig.ForceUpload {
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
			if AppConfig.DeleteFromHost {
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

	mediaKey, err := apiClient.CommitUpload(commitToken, fileInfo.Name(), sha1HashBytes, fileInfo.ModTime().Unix())
	if err != nil {
		return UploadResult{Error: fmt.Errorf("error commiting file: %w", err)}
	}

	if len(mediaKey) == 0 {
		return UploadResult{Error: fmt.Errorf("media key not received")}
	}

	if AppConfig.DeleteFromHost {
		if err := os.Remove(filePath); err != nil {
			slog.Warn("failed to delete file", "path", filePath, "error", err)
		} else {
			slog.Debug("deleted file", "path", filePath)
		}
	}

	return UploadResult{MediaKey: mediaKey, DedupKey: dedupKey}
}

func startUploadWorker(workerID int, workChan <-chan string, results chan<- FileUploadResult, cancel <-chan struct{}, wg *sync.WaitGroup, app *GooglePhotosCLI) {
	defer wg.Done()

	// Emit idle status initially
	app.EmitEvent("ThreadStatus", ThreadStatus{
		WorkerID: workerID,
		Status:   "idle",
		Message:  "Waiting for files...",
	})

	for path := range workChan {
		select {
		case <-cancel:
			app.EmitEvent("ThreadStatus", ThreadStatus{
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

			apiClient, err := api.NewApi(api.ApiConfig{
				AuthOverride: AuthOverride,
				Selected:     AppConfig.Selected,
				Credentials:  AppConfig.Credentials,
				Proxy:        AppConfig.Proxy,
				Saver:        AppConfig.Saver,
				UseQuota:     AppConfig.UseQuota,
			})
			if err != nil {
				results <- FileUploadResult{IsError: true, Error: err, Path: path}
				app.EmitEvent("ThreadStatus", ThreadStatus{
					WorkerID: workerID,
					Status:   "error",
					FilePath: path,
					FileName: filepath.Base(path),
					Message:  fmt.Sprintf("API error: %v", err),
				})
				continue
			}

			// Create callback from app interface
			callback := func(event string, data any) {
				app.EmitEvent(event, data)
			}
			result := uploadFileWithCallback(ctx, apiClient, path, workerID, callback)
			if result.Error != nil {
				results <- FileUploadResult{IsError: true, Error: result.Error, Path: path}
				app.EmitEvent("ThreadStatus", ThreadStatus{
					WorkerID: workerID,
					Status:   "error",
					FilePath: path,
					FileName: filepath.Base(path),
					Message:  fmt.Sprintf("Error: %v", result.Error),
				})
			} else {
				results <- FileUploadResult{IsError: false, IsExisting: result.IsExisting, Path: path, MediaKey: result.MediaKey, DedupKey: result.DedupKey}
				app.EmitEvent("ThreadStatus", ThreadStatus{
					WorkerID: workerID,
					Status:   "completed",
					FilePath: path,
					FileName: filepath.Base(path),
					Message:  "Completed",
				})
			}
			cancelUpload()

			// Mark as idle after completing file
			app.EmitEvent("ThreadStatus", ThreadStatus{
				WorkerID: workerID,
				Status:   "idle",
				Message:  "Waiting for next file...",
			})
		}
	}

	// Final idle status when no more work
	app.EmitEvent("ThreadStatus", ThreadStatus{
		WorkerID: workerID,
		Status:   "idle",
		Message:  "Finished",
	})
}
