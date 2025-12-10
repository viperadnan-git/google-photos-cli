package core

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/viperadnan-git/gogpm/internal/pb"

	"google.golang.org/protobuf/proto"
)

// MoveToTrash moves items to trash
// itemKeys can be either mediaKeys or dedupKeys (URL-safe base64 encoded SHA1 hashes)
func (a *Api) MoveToTrash(itemKeys []string) error {
	requestBody := pb.MoveToTrash{
		Field2:   1,
		ItemKeys: itemKeys,
		Field4:   1,
		Field8: &pb.MoveToTrash_Field8{
			Field4: &pb.MoveToTrash_Field8_Field4{
				Field2: &pb.MoveToTrash_Field8_Field4_Empty{},
				Field3: &pb.MoveToTrash_Field8_Field4_Field3{
					Field1: &pb.MoveToTrash_Field8_Field4_Empty{},
				},
				Field4: &pb.MoveToTrash_Field8_Field4_Empty{},
				Field5: &pb.MoveToTrash_Field8_Field4_Field5{
					Field1: &pb.MoveToTrash_Field8_Field4_Empty{},
				},
			},
		},
		Field9: &pb.MoveToTrash_Field9{
			Field1: 5,
			Field2: &pb.MoveToTrash_Field9_Field2{
				Field1: a.ClientVersionCode,
				Field2: strconv.FormatInt(a.AndroidAPIVersion, 10),
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
		"https://photosdata-pa.googleapis.com/6439526531001121323/17490284929287180316",
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

// RestoreFromTrash restores items from trash
// itemKeys can be either mediaKeys or dedupKeys (URL-safe base64 encoded SHA1 hashes)
func (a *Api) RestoreFromTrash(itemKeys []string) error {
	requestBody := pb.RestoreFromTrash{
		Field2:   3,
		ItemKeys: itemKeys,
		Field4:   2,
		Field8: &pb.RestoreFromTrash_Field8{
			Field4: &pb.RestoreFromTrash_Field8_Field4{
				Field2: &pb.RestoreFromTrash_Field8_Field4_Empty{},
				Field3: &pb.RestoreFromTrash_Field8_Field4_Field3{
					Field1: &pb.RestoreFromTrash_Field8_Field4_Empty{},
				},
			},
		},
		Field9: &pb.RestoreFromTrash_Field9{
			Field1: 5,
			Field2: &pb.RestoreFromTrash_Field9_Field2{
				Field1: a.ClientVersionCode,
				Field2: strconv.FormatInt(a.AndroidAPIVersion, 10),
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
		"https://photosdata-pa.googleapis.com/6439526531001121323/17490284929287180316",
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
