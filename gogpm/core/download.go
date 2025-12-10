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

// GetDownloadUrl gets the download URL for a media item
// Returns downloadURL and isEdited (true if the URL is for an edited version)
func (a *Api) GetDownloadUrl(mediaKey string) (downloadURL string, isEdited bool, err error) {
	requestBody := pb.GetDownloadUrl{
		Field1: &pb.GetDownloadUrl_Field1{
			Field1: &pb.GetDownloadUrl_Field1_Field1Inner{
				MediaKey: mediaKey,
			},
		},
		Field2: &pb.GetDownloadUrl_Field2{
			Field1: &pb.GetDownloadUrl_Field2_Field1Type{
				Field7: &pb.GetDownloadUrl_Field2_Field1Type_Field7Type{
					Field2: &pb.GetDownloadUrl_Field2_Field1Type_Field7Type_Field2Type{},
				},
			},
			Field5: &pb.GetDownloadUrl_Field2_Field5Type{
				Field2: &pb.GetDownloadUrl_Field2_Field5Type_Field2Type{},
				Field3: &pb.GetDownloadUrl_Field2_Field5Type_Field3Type{},
				Field5: &pb.GetDownloadUrl_Field2_Field5Type_Field5Inner{
					Field1: &pb.GetDownloadUrl_Field2_Field5Type_Field5Inner_Field1Type{},
					Field3: 0,
				},
			},
		},
	}

	serializedData, err := proto.Marshal(&requestBody)
	if err != nil {
		return "", false, fmt.Errorf("failed to marshal protobuf: %w", err)
	}

	bearerToken, err := a.BearerToken()
	if err != nil {
		return "", false, fmt.Errorf("failed to get bearer token: %w", err)
	}

	headers := a.CommonHeaders(bearerToken)

	req, err := http.NewRequest(
		"POST",
		"https://photosdata-pa.googleapis.com/$rpc/social.frontend.photos.preparedownloaddata.v1.PhotosPrepareDownloadDataService/PhotosPrepareDownload",
		bytes.NewReader(serializedData),
	)
	if err != nil {
		return "", false, fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := a.Client.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 404 {
			return "", false, fmt.Errorf("media item not found (status 404) - verify the media_key is correct")
		}
		body, _ := io.ReadAll(resp.Body)
		return "", false, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			return "", false, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer reader.(*gzip.Reader).Close()
	}

	bodyBytes, err := io.ReadAll(reader)
	if err != nil {
		return "", false, fmt.Errorf("failed to read response body: %w", err)
	}

	var response pb.GetDownloadUrlResponse
	if err := proto.Unmarshal(bodyBytes, &response); err != nil {
		return "", false, fmt.Errorf("failed to unmarshal protobuf: %w", err)
	}

	if response.GetField1() != nil && response.GetField1().GetField5() != nil && response.GetField1().GetField5().GetField3() != nil {
		downloadURL = response.GetField1().GetField5().GetField3().GetDownloadUrl()
		// The API returns a single download URL; isEdited indicates if edits were applied
		isEdited = response.GetField1().GetField5().GetField1() > 0
	}

	return downloadURL, isEdited, nil
}
