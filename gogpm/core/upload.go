package core

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"gpcli/pb"

	"google.golang.org/protobuf/proto"
)

// GetUploadToken obtains a file upload token from the Google Photos API
func (a *Api) GetUploadToken(sha1HashBase64 string, fileSize int64) (string, error) {
	requestBody := pb.GetUploadToken{
		F1:            2,
		F2:            2,
		F3:            1,
		F4:            3,
		FileSizeBytes: fileSize,
	}

	serializedData, err := proto.Marshal(&requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal protobuf: %w", err)
	}

	bearerToken, err := a.BearerToken()
	if err != nil {
		return "", fmt.Errorf("failed to get bearer token: %w", err)
	}

	headers := map[string]string{
		"Accept-Encoding":         "gzip",
		"Accept-Language":         a.Language,
		"Content-Type":            "application/x-protobuf",
		"User-Agent":              a.UserAgent,
		"Authorization":           "Bearer " + bearerToken,
		"X-Goog-Hash":             "sha1=" + sha1HashBase64,
		"X-Upload-Content-Length": strconv.Itoa(int(fileSize)),
	}

	req, err := http.NewRequest(
		"POST",
		"https://photos.googleapis.com/data/upload/uploadmedia/interactive",
		bytes.NewReader(serializedData),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := a.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	uploadToken := resp.Header.Get("X-GUploader-UploadID")
	if uploadToken == "" {
		return "", errors.New("response missing X-GUploader-UploadID header")
	}

	return uploadToken, nil
}

// FindRemoteMediaByHash checks the library for existing files with the given hash
func (a *Api) FindRemoteMediaByHash(sha1Hash []byte) (string, error) {
	requestBody := pb.HashCheck{
		Field1: &pb.HashCheckField1Type{
			Field1: &pb.HashCheckField1TypeField1Type{
				Sha1Hash: sha1Hash,
			},
			Field2: &pb.HashCheckField1TypeField2Type{},
		},
	}

	serializedData, err := proto.Marshal(&requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal protobuf: %w", err)
	}

	bearerToken, err := a.BearerToken()
	if err != nil {
		return "", fmt.Errorf("failed to get bearer token: %w", err)
	}

	headers := map[string]string{
		"Accept-Encoding": "gzip",
		"Accept-Language": a.Language,
		"Content-Type":    "application/x-protobuf",
		"User-Agent":      a.UserAgent,
		"Authorization":   "Bearer " + bearerToken,
	}

	req, err := http.NewRequest(
		"POST",
		"https://photosdata-pa.googleapis.com/6439526531001121323/5084965799730810217",
		bytes.NewReader(serializedData),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := a.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var reader io.Reader
	reader, err = gzip.NewReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer reader.(*gzip.Reader).Close()

	bodyBytes, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var response pb.RemoteMatches
	if err := proto.Unmarshal(bodyBytes, &response); err != nil {
		log.Fatalf("Failed to unmarshal protobuf: %v", err)
	}

	mediaKey := response.GetMediaKey()
	return mediaKey, nil
}

// UploadFile uploads a file to Google Photos using the provided upload token
func (a *Api) UploadFile(ctx context.Context, filePath string, uploadToken string) (*pb.CommitToken, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	uploadURL := "https://photos.googleapis.com/data/upload/uploadmedia/interactive?upload_id=" + uploadToken

	req, err := http.NewRequestWithContext(ctx, "PUT", uploadURL, file)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Enable chunked transfer encoding
	req.ContentLength = -1

	bearerToken, err := a.BearerToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get bearer token: %w", err)
	}

	headers := map[string]string{
		"Accept-Encoding": "gzip",
		"Accept-Language": a.Language,
		"User-Agent":      a.UserAgent,
		"Authorization":   "Bearer " + bearerToken,
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := a.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var commitToken pb.CommitToken
	if err := proto.Unmarshal(bodyBytes, &commitToken); err != nil {
		return nil, fmt.Errorf("failed to unmarshal protobuf: %w", err)
	}

	return &commitToken, nil
}

// CommitUpload commits the upload to Google Photos and returns the media key
// qualityStr: "original" or "storage-saver" (empty string uses Api default)
// useQuota: override Api default if true
func (a *Api) CommitUpload(
	commitToken *pb.CommitToken,
	fileName string,
	sha1Hash []byte,
	uploadTimestamp int64,
	qualityStr string,
	useQuota bool,
) (string, error) {
	if uploadTimestamp == 0 {
		uploadTimestamp = time.Now().Unix()
	}

	// Use defaults from Api if not overridden
	effectiveQuality := qualityStr
	if effectiveQuality == "" {
		effectiveQuality = a.Quality
	}
	effectiveUseQuota := useQuota || a.UseQuota

	// Determine model based on quality and quota settings
	model := a.Model
	var quality int64 = 3 // original
	if effectiveQuality == "storage-saver" {
		quality = 1
		model = "Pixel 2"
	}
	if effectiveUseQuota {
		model = "Pixel 8"
	}

	unknownConstant := int64(46000000)

	requestBody := pb.CommitUpload{
		Field1: &pb.CommitUploadField1Type{
			Field1: &pb.CommitUploadField1TypeField1Type{
				Field1: commitToken.Field1,
				Field2: commitToken.Field2,
			},
			FileName: fileName,
			Sha1Hash: sha1Hash,
			Field4: &pb.CommitUploadField1TypeField4Type{
				FileLastModifiedTimestamp: uploadTimestamp,
				Field2:                    unknownConstant,
			},
			Quality: quality,
			Field10: 1,
		},
		Field2: &pb.CommitUploadField2Type{
			Model:             model,
			Make:              a.Make,
			AndroidApiVersion: a.AndroidAPIVersion,
		},
		Field3: []byte{1, 3},
	}

	serializedData, err := proto.Marshal(&requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal protobuf: %w", err)
	}

	bearerToken, err := a.BearerToken()
	if err != nil {
		return "", fmt.Errorf("failed to get bearer token: %w", err)
	}

	headers := a.CommonHeaders(bearerToken)

	req, err := http.NewRequest(
		"POST",
		"https://photosdata-pa.googleapis.com/6439526531001121323/16538846908252377752",
		bytes.NewReader(serializedData),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := a.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer reader.(*gzip.Reader).Close()
	}

	bodyBytes, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var response pb.CommitUploadResponse
	if err := proto.Unmarshal(bodyBytes, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal protobuf: %w", err)
	}

	if response.GetField1() == nil || response.GetField1().GetField3() == nil {
		return "", fmt.Errorf("upload rejected by API: invalid response structure")
	}

	mediaKey := response.GetField1().GetField3().GetMediaKey()
	if mediaKey == "" {
		return "", fmt.Errorf("upload rejected by API: no media key returned")
	}

	return mediaKey, nil
}
