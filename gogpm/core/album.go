package core

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"time"

	"gpcli/pb"

	"google.golang.org/protobuf/proto"
)

// CreateAlbum creates a new album with the specified media keys
func (a *Api) CreateAlbum(albumName string, mediaKeys []string) (string, error) {
	mediaKeyRefs := make([]*pb.CreateAlbum_MediaKeyRef, len(mediaKeys))
	for i, key := range mediaKeys {
		mediaKeyRefs[i] = &pb.CreateAlbum_MediaKeyRef{
			Field1: &pb.CreateAlbum_MediaKeyRef_MediaKeyInner{
				MediaKey: key,
			},
		}
	}

	requestBody := pb.CreateAlbum{
		AlbumName: albumName,
		Timestamp: time.Now().Unix(),
		Field3:    1,
		MediaKeys: mediaKeyRefs,
		Field6:    &pb.CreateAlbum_Field6Type{},
		Field7:    &pb.CreateAlbum_Field7Type{Field1: 3},
		DeviceInfo: &pb.CreateAlbum_DeviceInfo{
			Model:             a.Model,
			Make:              a.Make,
			AndroidApiVersion: a.AndroidAPIVersion,
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

	headers := a.CommonHeaders(bearerToken)

	req, err := http.NewRequest(
		"POST",
		"https://photosdata-pa.googleapis.com/6439526531001121323/8386163679468898444",
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

	var response pb.CreateAlbumResponse
	if err := proto.Unmarshal(bodyBytes, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal protobuf: %w", err)
	}

	if response.GetField1() == nil {
		return "", fmt.Errorf("album creation failed: invalid response structure")
	}

	albumMediaKey := response.GetField1().GetAlbumMediaKey()
	if albumMediaKey == "" {
		return "", fmt.Errorf("album creation failed: no album media key returned")
	}

	return albumMediaKey, nil
}

// AddMediaToAlbum adds media items to an existing album
func (a *Api) AddMediaToAlbum(albumMediaKey string, mediaKeys []string) error {
	requestBody := pb.AddMediaToAlbum{
		MediaKeys:     mediaKeys,
		AlbumMediaKey: albumMediaKey,
		Field5:        &pb.AddMediaToAlbum_Field5Type{Field1: 2},
		DeviceInfo: &pb.AddMediaToAlbum_DeviceInfo{
			Model:             a.Model,
			Make:              a.Make,
			AndroidApiVersion: a.AndroidAPIVersion,
		},
		Timestamp: time.Now().Unix(),
	}

	serializedData, err := proto.Marshal(&requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal protobuf: %w", err)
	}

	bearerToken, err := a.BearerToken()
	if err != nil {
		return fmt.Errorf("failed to get bearer token: %w", err)
	}

	headers := a.CommonHeaders(bearerToken)

	req, err := http.NewRequest(
		"POST",
		"https://photosdata-pa.googleapis.com/6439526531001121323/484917746253879292",
		bytes.NewReader(serializedData),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := a.Client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
