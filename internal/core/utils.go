package core

import (
	"encoding/base64"
	"strings"
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
