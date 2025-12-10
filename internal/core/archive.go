package core

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/viperadnan-git/gogpm/internal/pb"

	"google.golang.org/protobuf/proto"
)

// SetArchived sets or removes the archived status for multiple items
// itemKeys can be either mediaKeys or dedupKeys (URL-safe base64 encoded SHA1 hashes)
// isArchived: true = archive, false = unarchive
func (a *Api) SetArchived(itemKeys []string, isArchived bool) error {
	// Action map: true (archive) = 1, false (unarchive) = 2
	var action int64 = 2
	if isArchived {
		action = 1
	}

	items := make([]*pb.SetArchived_ArchivedItem, len(itemKeys))
	for i, key := range itemKeys {
		items[i] = &pb.SetArchived_ArchivedItem{
			ItemKey: key,
			Action: &pb.SetArchived_ArchiveAction{
				Action: action,
			},
		}
	}

	requestBody := pb.SetArchived{
		Items:  items,
		Field3: 1,
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
		"https://photosdata-pa.googleapis.com/6439526531001121323/6715446385130606868",
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
