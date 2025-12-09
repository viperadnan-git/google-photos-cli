package api

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"

	"gpcli/pb"

	"google.golang.org/protobuf/proto"
)

// ToURLSafeBase64 converts a standard base64 string to URL-safe base64
// This is used to create dedup_keys from SHA1 hashes
func ToURLSafeBase64(base64Hash string) string {
	// Replace + with -, / with _, and remove trailing =
	result := strings.ReplaceAll(base64Hash, "+", "-")
	result = strings.ReplaceAll(result, "/", "_")
	result = strings.TrimRight(result, "=")
	return result
}

// SHA1ToDedupeKey converts a SHA1 hash (raw bytes) to a dedup_key
func SHA1ToDedupeKey(sha1Hash []byte) string {
	base64Hash := base64.StdEncoding.EncodeToString(sha1Hash)
	return ToURLSafeBase64(base64Hash)
}

// SetArchived sets or removes the archived status for multiple items
// dedupKeys are URL-safe base64 encoded SHA1 hashes of the files
// isArchived: true = archive, false = unarchive
func (a *Api) SetArchived(dedupKeys []string, isArchived bool) error {
	// Action map: true (archive) = 1, false (unarchive) = 2
	var action int64 = 2
	if isArchived {
		action = 1
	}

	items := make([]*pb.SetArchived_ArchivedItem, len(dedupKeys))
	for i, key := range dedupKeys {
		items[i] = &pb.SetArchived_ArchivedItem{
			DedupKey: key,
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
