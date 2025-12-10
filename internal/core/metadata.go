package core

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/viperadnan-git/gogpm/internal/pb"

	"google.golang.org/protobuf/proto"
)

// SetCaption sets the caption for a media item
// itemKey can be either mediaKey or dedupKey
func (a *Api) SetCaption(itemKey, caption string) error {
	requestBody := pb.SetCaption{
		Caption: caption,
		ItemKey: itemKey,
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
		"https://photosdata-pa.googleapis.com/6439526531001121323/1552790390512470739",
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

// SetFavourite sets or removes the favourite status for a media item
// itemKey can be either mediaKey or dedupKey
// isFavourite: true = favourite, false = unfavourite
func (a *Api) SetFavourite(itemKey string, isFavourite bool) error {
	// Action map: true (favourite) = 1, false (unfavourite) = 2
	var action int64 = 2
	if isFavourite {
		action = 1
	}

	requestBody := pb.SetFavourite{
		Field1: &pb.SetFavourite_Field1{
			ItemKey: itemKey,
		},
		Field2: &pb.SetFavourite_Field2{
			Action: action,
		},
		Field3: &pb.SetFavourite_Field3{
			Field1: &pb.SetFavourite_Field3_Field1Inner{
				Field19: &pb.SetFavourite_Field3_Field1Inner_Field19{},
			},
		},
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
		"https://photosdata-pa.googleapis.com/6439526531001121323/5144645502632292153",
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
