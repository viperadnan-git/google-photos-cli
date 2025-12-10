package core

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"

	"gpcli/pb"

	"google.golang.org/protobuf/proto"
)

// GetDownloadUrls gets the download URLs for a media item
// Returns editedURL (file with edits applied) and originalURL (original file)
func (a *Api) GetDownloadUrls(mediaKey string) (editedURL, originalURL string, err error) {
	requestBody := pb.GetDownloadUrls{
		Field1: &pb.GetDownloadUrls_Field1{
			Field1: &pb.GetDownloadUrls_Field1_Field1Inner{
				MediaKey: mediaKey,
			},
		},
		Field2: &pb.GetDownloadUrls_Field2{
			Field1: &pb.GetDownloadUrls_Field2_Field1Type{
				Field7: &pb.GetDownloadUrls_Field2_Field1Type_Field7Type{
					Field2: &pb.GetDownloadUrls_Field2_Field1Type_Field7Type_Field2Type{},
				},
			},
			Field5: &pb.GetDownloadUrls_Field2_Field5Type{
				Field2: &pb.GetDownloadUrls_Field2_Field5Type_Field2Type{},
				Field3: &pb.GetDownloadUrls_Field2_Field5Type_Field3Type{},
				Field5: &pb.GetDownloadUrls_Field2_Field5Type_Field5Inner{
					Field1: &pb.GetDownloadUrls_Field2_Field5Type_Field5Inner_Field1Type{},
					Field3: 0,
				},
			},
		},
	}

	serializedData, err := proto.Marshal(&requestBody)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal protobuf: %w", err)
	}

	bearerToken, err := a.BearerToken()
	if err != nil {
		return "", "", fmt.Errorf("failed to get bearer token: %w", err)
	}

	headers := a.CommonHeaders(bearerToken)

	req, err := http.NewRequest(
		"POST",
		"https://photosdata-pa.googleapis.com/$rpc/social.frontend.photos.preparedownloaddata.v1.PhotosPrepareDownloadDataService/PhotosPrepareDownload",
		bytes.NewReader(serializedData),
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := a.Client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 404 {
			return "", "", fmt.Errorf("media item not found (status 404) - verify the media_key is correct")
		}
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			return "", "", fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer reader.(*gzip.Reader).Close()
	}

	bodyBytes, err := io.ReadAll(reader)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response body: %w", err)
	}

	var response pb.GetDownloadUrlsResponse
	if err := proto.Unmarshal(bodyBytes, &response); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal protobuf: %w", err)
	}

	if response.GetField1() != nil && response.GetField1().GetField5() != nil && response.GetField1().GetField5().GetField3() != nil {
		downloadURL := response.GetField1().GetField5().GetField3().GetDownloadUrl()
		// The API returns a single download URL for both edited and original
		editedURL = downloadURL
		originalURL = downloadURL
	}

	return editedURL, originalURL, nil
}
