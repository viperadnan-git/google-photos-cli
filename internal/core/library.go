package core

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/viperadnan-git/go-gpm/internal/pb"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

// MediaItemCopy represents a copy of a media item (e.g., shared to an album)
type MediaItemCopy struct {
	MediaKey string `json:"media_key"`
	AlbumID  string `json:"album_id"`
}

// MediaItem represents a media item (photo/video) in the library
type MediaItem struct {
	MediaKey           string          `json:"media_key"`
	FileName           string          `json:"file_name"`
	DedupKey           string          `json:"dedup_key"`
	Type               int             `json:"type"` // 1=photo, 2=video
	SizeBytes          uint64          `json:"size_bytes"`
	UTCTimestamp       uint64          `json:"utc_timestamp"`
	ServerCreationTime uint64          `json:"server_creation_timestamp"`
	TimezoneOffset     int64           `json:"timezone_offset"`
	ContentVersion     uint64          `json:"content_version"`
	Width              int             `json:"width,omitempty"`
	Height             int             `json:"height,omitempty"`
	ThumbnailURL       string          `json:"thumbnail_url,omitempty"`
	DownloadURL        string          `json:"download_url,omitempty"`
	Duration           uint64          `json:"duration,omitempty"` // for videos, in ms
	IsArchived         bool            `json:"is_archived"`
	IsFavorite         bool            `json:"is_favorite"`
	IsLocked           bool            `json:"is_locked"`
	IsOriginalQuality  bool            `json:"is_original_quality"`
	QuotaChargedBytes  uint64          `json:"quota_charged_bytes"`
	AlbumID            string          `json:"album_id,omitempty"` // Album this item belongs to (for original items)
	Caption            string          `json:"caption,omitempty"`
	Copies             []MediaItemCopy `json:"copies,omitempty"` // Non-canonical copies of this item
}

// AlbumMediaItem represents a media item inside an album
type AlbumMediaItem struct {
	MediaKey         string `json:"media_key"`          // Key of the item in this album (copy)
	OriginalMediaKey string `json:"original_media_key"` // Key of the canonical/original item
	FileName         string `json:"file_name"`
	DedupKey         string `json:"dedup_key"`
}

// AlbumItem represents an album in the library
type AlbumItem struct {
	AlbumKey       string           `json:"album_key"`
	Title          string           `json:"title"`
	ItemCount      int              `json:"item_count"`
	Type           int              `json:"type"` // 1=user album
	CoverMediaKey  string           `json:"cover_media_key,omitempty"`
	CoverURL       string           `json:"cover_url,omitempty"`
	LastActivityMs uint64           `json:"last_activity_ms,omitempty"`
	MediaItems     []AlbumMediaItem `json:"media_items,omitempty"`
}

// LibraryStateResponse contains parsed response data from library state APIs
type LibraryStateResponse struct {
	RawBytes         []byte      `json:"-"` // Raw protobuf response bytes (excluded from JSON)
	StateToken       string      `json:"state_token"`
	NextPageToken    string      `json:"next_page_token,omitempty"`
	MediaItems       []MediaItem `json:"media_items,omitempty"`
	Albums           []AlbumItem `json:"albums,omitempty"`
	DeletedMediaKeys []string    `json:"deleted_media_keys,omitempty"`
}

// GetLibraryState fetches the library state using a state token
func (a *Api) GetLibraryState(stateToken string) (*LibraryStateResponse, error) {
	requestBody := buildLibraryStateRequest(stateToken, "")
	return a.doLibraryRequest(requestBody)
}

// GetLibraryPageInit fetches initial library page using a page token
func (a *Api) GetLibraryPageInit(pageToken string) (*LibraryStateResponse, error) {
	requestBody := buildLibraryPageInitRequest(pageToken)
	return a.doLibraryRequest(requestBody)
}

// GetLibraryPage fetches library page using both page and state tokens
func (a *Api) GetLibraryPage(pageToken, stateToken string) (*LibraryStateResponse, error) {
	requestBody := buildLibraryPageRequest(pageToken, stateToken)
	return a.doLibraryRequest(requestBody)
}

func (a *Api) doLibraryRequest(requestBody []byte) (*LibraryStateResponse, error) {
	bodyBytes, _, err := a.DoRequest(
		"https://photosdata-pa.googleapis.com/6439526531001121323/18047484249733410717",
		bytes.NewReader(requestBody),
		WithAuth(),
		WithCommonHeaders(),
		WithStatusCheck(),
	)
	if err != nil {
		return nil, fmt.Errorf("library request failed: %w", err)
	}

	resp := &LibraryStateResponse{RawBytes: bodyBytes}

	// Parse using generated protobuf
	var pbResp pb.LibraryStateResponse
	if err := proto.Unmarshal(bodyBytes, &pbResp); err == nil && pbResp.Data != nil {
		resp.parseFromProto(pbResp.Data)
	}

	return resp, nil
}

// parseFromProto populates the response from parsed protobuf
func (r *LibraryStateResponse) parseFromProto(data *pb.LibraryStateData) {
	r.StateToken = data.StateToken
	r.NextPageToken = data.NextPageToken

	// First pass: parse all items, separate canonical and non-canonical
	type parsedItem struct {
		item        *MediaItem
		isCanonical bool
		albumID     string
	}
	var allItems []parsedItem
	// Map dedup_key -> index in allItems (for canonical items)
	dedupToCanonicalIdx := make(map[string]int)
	// Map dedup_key -> list of copies
	dedupToCopies := make(map[string][]MediaItemCopy)
	// Map album_id -> list of album media items
	collectionItems := make(map[string][]AlbumMediaItem)

	for _, pbItem := range data.MediaItems {
		item, isCanonical, albumID := parseMediaItemFromProtoEx(pbItem)
		if item != nil {
			idx := len(allItems)
			allItems = append(allItems, parsedItem{item: item, isCanonical: isCanonical, albumID: albumID})
			if isCanonical && item.DedupKey != "" {
				dedupToCanonicalIdx[item.DedupKey] = idx
			}
		}
	}

	// Second pass: build copies map and album media items
	for _, p := range allItems {
		if !p.isCanonical && p.item.DedupKey != "" {
			copy := MediaItemCopy{
				MediaKey: p.item.MediaKey,
				AlbumID:  p.albumID,
			}
			dedupToCopies[p.item.DedupKey] = append(dedupToCopies[p.item.DedupKey], copy)

			// Build album media items
			if p.albumID != "" {
				canonicalIdx, hasCanonical := dedupToCanonicalIdx[p.item.DedupKey]
				var originalKey string
				if hasCanonical {
					originalKey = allItems[canonicalIdx].item.MediaKey
				}
				albumMediaItem := AlbumMediaItem{
					MediaKey:         p.item.MediaKey,
					OriginalMediaKey: originalKey,
					FileName:         p.item.FileName,
					DedupKey:         p.item.DedupKey,
				}
				collectionItems[p.albumID] = append(collectionItems[p.albumID], albumMediaItem)
			}
		}
	}

	// Third pass: attach copies to canonical items and build final list
	for _, p := range allItems {
		if p.isCanonical {
			if copies, ok := dedupToCopies[p.item.DedupKey]; ok {
				p.item.Copies = copies
			}
			p.item.AlbumID = p.albumID
			r.MediaItems = append(r.MediaItems, *p.item)
		}
	}

	// Parse albums
	for _, pbAlbum := range data.Albums {
		album := parseAlbumItemFromProto(pbAlbum, collectionItems)
		if album != nil {
			r.Albums = append(r.Albums, *album)
		}
	}

	// Parse deletions
	for _, pbDeletion := range data.Deletions {
		if pbDeletion.Info != nil && pbDeletion.Info.DeletionType == 1 && pbDeletion.Info.Media != nil {
			r.DeletedMediaKeys = append(r.DeletedMediaKeys, pbDeletion.Info.Media.MediaKey)
		}
	}
}

// parseMediaItemFromProtoEx converts a protobuf MediaItemData to MediaItem
// Returns (item, isCanonical, albumID)
func parseMediaItemFromProtoEx(pbItem *pb.MediaItemData) (*MediaItem, bool, string) {
	if pbItem == nil {
		return nil, false, ""
	}

	item := &MediaItem{
		MediaKey: pbItem.MediaKey,
	}
	isCanonical := true
	var albumID string

	// Parse metadata
	if m := pbItem.Metadata; m != nil {
		item.FileName = m.FileName
		item.UTCTimestamp = m.UtcTimestamp
		item.TimezoneOffset = int64(m.TimezoneOffset)
		item.ServerCreationTime = m.ServerCreationTimestamp
		item.SizeBytes = m.SizeBytes
		item.ContentVersion = m.ContentVersion

		// Album ID (collection)
		if m.Collection != nil {
			albumID = m.Collection.CollectionId
		}

		// Dedup key
		if m.Dedup != nil {
			item.DedupKey = m.Dedup.DedupKey
		}

		// Archive status
		if m.Archive != nil {
			item.IsArchived = m.Archive.Status == 1
		}

		// Favorite status
		if m.Favorite != nil {
			item.IsFavorite = m.Favorite.Status == 1
		}

		// Quota info
		if m.Quota != nil {
			item.QuotaChargedBytes = m.Quota.QuotaChargedBytes
			item.IsOriginalQuality = m.Quota.QualityType == 2
		}

		// Lock status
		if m.Lock != nil {
			item.IsLocked = m.Lock.Status == 1
		}

		// Check for property 27 (non-canonical marker)
		for _, prop := range m.Properties {
			if prop.PropertyId == 27 {
				isCanonical = false
				break
			}
		}
	}

	// Parse type info
	if t := pbItem.TypeInfo; t != nil {
		item.Type = int(t.MediaType)

		// Photo info
		if t.Photo != nil && t.Photo.Details != nil {
			item.ThumbnailURL = t.Photo.Details.ThumbnailUrl
			if t.Photo.Details.Dimensions != nil {
				item.Width = int(t.Photo.Details.Dimensions.Width)
				item.Height = int(t.Photo.Details.Dimensions.Height)
			}
		}

		// Video info
		if t.Video != nil {
			if t.Video.Thumbnail != nil {
				item.ThumbnailURL = t.Video.Thumbnail.Url
			}
			item.DownloadURL = t.Video.DownloadUrl
			if t.Video.Details != nil {
				item.Duration = t.Video.Details.DurationMs
				item.Width = int(t.Video.Details.Width)
				item.Height = int(t.Video.Details.Height)
			}
		}
	}

	// Alternative media key
	if pbItem.KeyInfo != nil && item.MediaKey == "" {
		item.MediaKey = pbItem.KeyInfo.MediaKey
	}

	if item.MediaKey == "" && item.FileName == "" {
		return nil, false, ""
	}

	return item, isCanonical, albumID
}

// parseAlbumItemFromProto converts a protobuf AlbumData to AlbumItem
func parseAlbumItemFromProto(pbAlbum *pb.AlbumData, collectionItems map[string][]AlbumMediaItem) *AlbumItem {
	if pbAlbum == nil || pbAlbum.AlbumKey == "" {
		return nil
	}

	album := &AlbumItem{
		AlbumKey: pbAlbum.AlbumKey,
	}

	if m := pbAlbum.Metadata; m != nil {
		album.Title = m.Title
		album.Type = int(m.AlbumType)
		if m.CoverItem != nil {
			album.CoverMediaKey = m.CoverItem.MediaKey
		}
	}

	// Media items from collection mapping
	if items, ok := collectionItems[album.AlbumKey]; ok {
		album.MediaItems = items
		album.ItemCount = len(items)
	}

	return album
}

// ToJSON converts the response to JSON string for viewing
func (r *LibraryStateResponse) ToJSON() (string, error) {
	jsonBytes, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

// ToRawJSON converts raw protobuf bytes to JSON string for debugging
func (r *LibraryStateResponse) ToRawJSON() (string, error) {
	parsed, err := parseProtobufMessage(r.RawBytes)
	if err != nil {
		return "", err
	}
	jsonBytes, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

// parseProtobufMessage parses raw protobuf bytes into a map (for debugging)
func parseProtobufMessage(data []byte) (map[string]any, error) {
	result := make(map[string]any)

	for len(data) > 0 {
		fieldNum, wireType, n := protowire.ConsumeTag(data)
		if n < 0 {
			return nil, fmt.Errorf("invalid tag at position %d", len(data))
		}
		data = data[n:]

		fieldKey := fmt.Sprintf("%d", fieldNum)
		var value any
		var consumed int

		switch wireType {
		case protowire.VarintType:
			v, n := protowire.ConsumeVarint(data)
			if n < 0 {
				return nil, fmt.Errorf("invalid varint")
			}
			value = v
			consumed = n

		case protowire.Fixed64Type:
			v, n := protowire.ConsumeFixed64(data)
			if n < 0 {
				return nil, fmt.Errorf("invalid fixed64")
			}
			value = v
			consumed = n

		case protowire.BytesType:
			v, n := protowire.ConsumeBytes(data)
			if n < 0 {
				return nil, fmt.Errorf("invalid bytes")
			}
			if nested, err := parseProtobufMessage(v); err == nil && len(nested) > 0 {
				value = nested
			} else if isPrintable(v) {
				value = string(v)
			} else {
				value = fmt.Sprintf("bytes[%d]", len(v))
			}
			consumed = n

		case protowire.StartGroupType:
			return nil, fmt.Errorf("groups not supported")

		case protowire.Fixed32Type:
			v, n := protowire.ConsumeFixed32(data)
			if n < 0 {
				return nil, fmt.Errorf("invalid fixed32")
			}
			value = v
			consumed = n

		default:
			return nil, fmt.Errorf("unknown wire type: %d", wireType)
		}

		data = data[consumed:]

		if existing, ok := result[fieldKey]; ok {
			switch e := existing.(type) {
			case []any:
				result[fieldKey] = append(e, value)
			default:
				result[fieldKey] = []any{e, value}
			}
		} else {
			result[fieldKey] = value
		}
	}

	return result, nil
}

func isPrintable(b []byte) bool {
	if len(b) == 0 {
		return true
	}
	for _, c := range b {
		if c < 32 || c > 126 {
			if c != '\n' && c != '\r' && c != '\t' {
				return false
			}
		}
	}
	return true
}

// Helper functions to build protobuf wire format

func appendTag(b []byte, fieldNum protowire.Number, wireType protowire.Type) []byte {
	return protowire.AppendTag(b, fieldNum, wireType)
}

func appendVarint(b []byte, v uint64) []byte {
	return protowire.AppendVarint(b, v)
}

func appendString(b []byte, s string) []byte {
	return protowire.AppendString(b, s)
}

func appendBytes(b []byte, v []byte) []byte {
	return protowire.AppendBytes(b, v)
}

func appendMessage(b []byte, fieldNum protowire.Number, msg []byte) []byte {
	b = appendTag(b, fieldNum, protowire.BytesType)
	return appendBytes(b, msg)
}

func appendEmptyMessage(b []byte, fieldNum protowire.Number) []byte {
	return appendMessage(b, fieldNum, []byte{})
}

func appendIntField(b []byte, fieldNum protowire.Number, v uint64) []byte {
	b = appendTag(b, fieldNum, protowire.VarintType)
	return appendVarint(b, v)
}

func appendStringField(b []byte, fieldNum protowire.Number, s string) []byte {
	b = appendTag(b, fieldNum, protowire.BytesType)
	return appendString(b, s)
}

// buildField1_1_1 builds the deeply nested field 1.1.1 structure
func buildField1_1_1() []byte {
	var b []byte
	b = appendEmptyMessage(b, 1)
	b = appendEmptyMessage(b, 3)
	b = appendEmptyMessage(b, 4)

	var f5 []byte
	f5 = appendEmptyMessage(f5, 1)
	f5 = appendEmptyMessage(f5, 2)
	f5 = appendEmptyMessage(f5, 3)
	f5 = appendEmptyMessage(f5, 4)
	f5 = appendEmptyMessage(f5, 5)
	f5 = appendEmptyMessage(f5, 7)
	b = appendMessage(b, 5, f5)

	b = appendEmptyMessage(b, 6)

	var f7 []byte
	f7 = appendEmptyMessage(f7, 2)
	b = appendMessage(b, 7, f7)

	b = appendEmptyMessage(b, 15)
	b = appendEmptyMessage(b, 16)
	b = appendEmptyMessage(b, 17)
	b = appendEmptyMessage(b, 19)
	b = appendEmptyMessage(b, 20)

	var f21 []byte
	var f21_5 []byte
	f21_5 = appendEmptyMessage(f21_5, 3)
	f21 = appendMessage(f21, 5, f21_5)
	f21 = appendEmptyMessage(f21, 6)
	b = appendMessage(b, 21, f21)

	b = appendEmptyMessage(b, 25)

	var f30 []byte
	f30 = appendEmptyMessage(f30, 2)
	b = appendMessage(b, 30, f30)

	b = appendEmptyMessage(b, 31)
	b = appendEmptyMessage(b, 32)

	var f33 []byte
	f33 = appendEmptyMessage(f33, 1)
	b = appendMessage(b, 33, f33)

	b = appendEmptyMessage(b, 34)
	b = appendEmptyMessage(b, 36)
	b = appendEmptyMessage(b, 37)
	b = appendEmptyMessage(b, 38)
	b = appendEmptyMessage(b, 39)
	b = appendEmptyMessage(b, 40)
	b = appendEmptyMessage(b, 41)

	return b
}

// buildField1_1_5 builds the field 1.1.5 structure
func buildField1_1_5() []byte {
	var b []byte

	var f2 []byte
	var f2_2 []byte
	var f2_2_3 []byte
	f2_2_3 = appendEmptyMessage(f2_2_3, 2)
	f2_2 = appendMessage(f2_2, 3, f2_2_3)
	var f2_2_4 []byte
	f2_2_4 = appendEmptyMessage(f2_2_4, 2)
	f2_2 = appendMessage(f2_2, 4, f2_2_4)
	f2 = appendMessage(f2, 2, f2_2)
	var f2_4 []byte
	var f2_4_2 []byte
	f2_4_2 = appendIntField(f2_4_2, 2, 1)
	f2_4 = appendMessage(f2_4, 2, f2_4_2)
	f2 = appendMessage(f2, 4, f2_4)
	var f2_5 []byte
	f2_5 = appendEmptyMessage(f2_5, 2)
	f2 = appendMessage(f2, 5, f2_5)
	f2 = appendIntField(f2, 6, 1)
	b = appendMessage(b, 2, f2)

	var f3 []byte
	var f3_2 []byte
	f3_2 = appendEmptyMessage(f3_2, 3)
	f3_2 = appendEmptyMessage(f3_2, 4)
	f3 = appendMessage(f3, 2, f3_2)
	var f3_3 []byte
	f3_3 = appendEmptyMessage(f3_3, 2)
	var f3_3_3 []byte
	f3_3_3 = appendIntField(f3_3_3, 2, 1)
	f3_3 = appendMessage(f3_3, 3, f3_3_3)
	f3 = appendMessage(f3, 3, f3_3)
	f3 = appendEmptyMessage(f3, 4)
	var f3_5 []byte
	var f3_5_2 []byte
	f3_5_2 = appendIntField(f3_5_2, 2, 1)
	f3_5 = appendMessage(f3_5, 2, f3_5_2)
	f3 = appendMessage(f3, 5, f3_5)
	f3 = appendEmptyMessage(f3, 7)
	b = appendMessage(b, 3, f3)

	var f4 []byte
	var f4_2 []byte
	f4_2 = appendEmptyMessage(f4_2, 2)
	f4 = appendMessage(f4, 2, f4_2)
	b = appendMessage(b, 4, f4)

	var f5 []byte
	var f5_1 []byte
	var f5_1_2 []byte
	f5_1_2 = appendEmptyMessage(f5_1_2, 3)
	f5_1_2 = appendEmptyMessage(f5_1_2, 4)
	f5_1 = appendMessage(f5_1, 2, f5_1_2)
	var f5_1_3 []byte
	f5_1_3 = appendEmptyMessage(f5_1_3, 2)
	var f5_1_3_3 []byte
	f5_1_3_3 = appendIntField(f5_1_3_3, 2, 1)
	f5_1_3 = appendMessage(f5_1_3, 3, f5_1_3_3)
	f5_1 = appendMessage(f5_1, 3, f5_1_3)
	f5 = appendMessage(f5, 1, f5_1)
	f5 = appendIntField(f5, 3, 1)
	b = appendMessage(b, 5, f5)

	return b
}

// buildField1_1 builds the field 1.1 structure
func buildField1_1() []byte {
	var b []byte
	b = appendMessage(b, 1, buildField1_1_1())
	b = appendMessage(b, 5, buildField1_1_5())
	b = appendEmptyMessage(b, 8)

	var f9 []byte
	f9 = appendEmptyMessage(f9, 2)
	var f9_3 []byte
	f9_3 = appendEmptyMessage(f9_3, 1)
	f9_3 = appendEmptyMessage(f9_3, 2)
	f9 = appendMessage(f9, 3, f9_3)
	var f9_4 []byte
	var f9_4_1 []byte
	var f9_4_1_3 []byte
	var f9_4_1_3_1 []byte
	var f9_4_1_3_1_1 []byte
	var f9_4_1_3_1_1_5 []byte
	f9_4_1_3_1_1_5 = appendEmptyMessage(f9_4_1_3_1_1_5, 1)
	f9_4_1_3_1_1 = appendMessage(f9_4_1_3_1_1, 5, f9_4_1_3_1_1_5)
	f9_4_1_3_1_1 = appendEmptyMessage(f9_4_1_3_1_1, 6)
	f9_4_1_3_1 = appendMessage(f9_4_1_3_1, 1, f9_4_1_3_1_1)
	f9_4_1_3_1 = appendEmptyMessage(f9_4_1_3_1, 2)
	var f9_4_1_3_1_3 []byte
	var f9_4_1_3_1_3_1 []byte
	var f9_4_1_3_1_3_1_5 []byte
	f9_4_1_3_1_3_1_5 = appendEmptyMessage(f9_4_1_3_1_3_1_5, 1)
	f9_4_1_3_1_3_1 = appendMessage(f9_4_1_3_1_3_1, 5, f9_4_1_3_1_3_1_5)
	f9_4_1_3_1_3_1 = appendEmptyMessage(f9_4_1_3_1_3_1, 6)
	f9_4_1_3_1_3 = appendMessage(f9_4_1_3_1_3, 1, f9_4_1_3_1_3_1)
	f9_4_1_3_1_3 = appendEmptyMessage(f9_4_1_3_1_3, 2)
	f9_4_1_3_1 = appendMessage(f9_4_1_3_1, 3, f9_4_1_3_1_3)
	f9_4_1_3 = appendMessage(f9_4_1_3, 1, f9_4_1_3_1)
	f9_4_1 = appendMessage(f9_4_1, 3, f9_4_1_3)
	var f9_4_1_4 []byte
	var f9_4_1_4_1 []byte
	f9_4_1_4_1 = appendEmptyMessage(f9_4_1_4_1, 2)
	f9_4_1_4 = appendMessage(f9_4_1_4, 1, f9_4_1_4_1)
	f9_4_1 = appendMessage(f9_4_1, 4, f9_4_1_4)
	f9_4 = appendMessage(f9_4, 1, f9_4_1)
	f9 = appendMessage(f9, 4, f9_4)
	b = appendMessage(b, 9, f9)

	var f11 []byte
	f11 = appendEmptyMessage(f11, 2)
	f11 = appendEmptyMessage(f11, 3)
	var f11_4 []byte
	var f11_4_2 []byte
	f11_4_2 = appendIntField(f11_4_2, 1, 1)
	f11_4_2 = appendIntField(f11_4_2, 2, 2)
	f11_4 = appendMessage(f11_4, 2, f11_4_2)
	f11 = appendMessage(f11, 4, f11_4)
	b = appendMessage(b, 11, f11)

	b = appendEmptyMessage(b, 12)

	var f14 []byte
	f14 = appendEmptyMessage(f14, 2)
	f14 = appendEmptyMessage(f14, 3)
	var f14_4 []byte
	var f14_4_2 []byte
	f14_4_2 = appendIntField(f14_4_2, 1, 1)
	f14_4_2 = appendIntField(f14_4_2, 2, 2)
	f14_4 = appendMessage(f14_4, 2, f14_4_2)
	f14 = appendMessage(f14, 4, f14_4)
	b = appendMessage(b, 14, f14)

	var f15 []byte
	f15 = appendEmptyMessage(f15, 1)
	f15 = appendEmptyMessage(f15, 4)
	b = appendMessage(b, 15, f15)

	var f17 []byte
	f17 = appendEmptyMessage(f17, 1)
	f17 = appendEmptyMessage(f17, 4)
	b = appendMessage(b, 17, f17)

	var f19 []byte
	f19 = appendEmptyMessage(f19, 2)
	f19 = appendEmptyMessage(f19, 3)
	var f19_4 []byte
	var f19_4_2 []byte
	f19_4_2 = appendIntField(f19_4_2, 1, 1)
	f19_4_2 = appendIntField(f19_4_2, 2, 2)
	f19_4 = appendMessage(f19_4, 2, f19_4_2)
	f19 = appendMessage(f19, 4, f19_4)
	b = appendMessage(b, 19, f19)

	b = appendEmptyMessage(b, 22)
	b = appendEmptyMessage(b, 23)

	return b
}

// buildField1_2 builds the field 1.2 structure
func buildField1_2() []byte {
	var b []byte

	var f1 []byte
	f1 = appendEmptyMessage(f1, 2)
	f1 = appendEmptyMessage(f1, 3)
	f1 = appendEmptyMessage(f1, 4)
	f1 = appendEmptyMessage(f1, 5)
	var f1_6 []byte
	f1_6 = appendEmptyMessage(f1_6, 1)
	f1_6 = appendEmptyMessage(f1_6, 2)
	f1_6 = appendEmptyMessage(f1_6, 3)
	f1_6 = appendEmptyMessage(f1_6, 4)
	f1_6 = appendEmptyMessage(f1_6, 5)
	f1_6 = appendEmptyMessage(f1_6, 7)
	f1 = appendMessage(f1, 6, f1_6)
	f1 = appendEmptyMessage(f1, 7)
	f1 = appendEmptyMessage(f1, 8)
	f1 = appendEmptyMessage(f1, 10)
	f1 = appendEmptyMessage(f1, 12)
	var f1_13 []byte
	f1_13 = appendEmptyMessage(f1_13, 2)
	f1_13 = appendEmptyMessage(f1_13, 3)
	f1 = appendMessage(f1, 13, f1_13)
	var f1_15 []byte
	f1_15 = appendEmptyMessage(f1_15, 1)
	f1 = appendMessage(f1, 15, f1_15)
	f1 = appendEmptyMessage(f1, 18)
	b = appendMessage(b, 1, f1)

	var f4 []byte
	f4 = appendEmptyMessage(f4, 1)
	b = appendMessage(b, 4, f4)

	b = appendEmptyMessage(b, 9)

	var f11 []byte
	var f11_1 []byte
	f11_1 = appendEmptyMessage(f11_1, 1)
	f11_1 = appendEmptyMessage(f11_1, 4)
	f11_1 = appendEmptyMessage(f11_1, 5)
	f11_1 = appendEmptyMessage(f11_1, 6)
	f11_1 = appendEmptyMessage(f11_1, 9)
	f11 = appendMessage(f11, 1, f11_1)
	b = appendMessage(b, 11, f11)

	var f14 []byte
	var f14_1 []byte
	var f14_1_1 []byte
	f14_1_1 = appendEmptyMessage(f14_1_1, 1)
	var f14_1_1_2 []byte
	var f14_1_1_2_2 []byte
	var f14_1_1_2_2_1 []byte
	f14_1_1_2_2_1 = appendEmptyMessage(f14_1_1_2_2_1, 1)
	f14_1_1_2_2 = appendMessage(f14_1_1_2_2, 1, f14_1_1_2_2_1)
	f14_1_1_2_2 = appendEmptyMessage(f14_1_1_2_2, 3)
	f14_1_1_2 = appendMessage(f14_1_1_2, 2, f14_1_1_2_2)
	f14_1_1 = appendMessage(f14_1_1, 2, f14_1_1_2)
	var f14_1_1_3 []byte
	var f14_1_1_3_4 []byte
	var f14_1_1_3_4_1 []byte
	f14_1_1_3_4_1 = appendEmptyMessage(f14_1_1_3_4_1, 1)
	f14_1_1_3_4 = appendMessage(f14_1_1_3_4, 1, f14_1_1_3_4_1)
	f14_1_1_3_4 = appendEmptyMessage(f14_1_1_3_4, 3)
	f14_1_1_3 = appendMessage(f14_1_1_3, 4, f14_1_1_3_4)
	var f14_1_1_3_5 []byte
	var f14_1_1_3_5_1 []byte
	f14_1_1_3_5_1 = appendEmptyMessage(f14_1_1_3_5_1, 1)
	f14_1_1_3_5 = appendMessage(f14_1_1_3_5, 1, f14_1_1_3_5_1)
	f14_1_1_3_5 = appendEmptyMessage(f14_1_1_3_5, 3)
	f14_1_1_3 = appendMessage(f14_1_1_3, 5, f14_1_1_3_5)
	f14_1_1 = appendMessage(f14_1_1, 3, f14_1_1_3)
	f14_1 = appendMessage(f14_1, 1, f14_1_1)
	f14_1 = appendEmptyMessage(f14_1, 2)
	f14 = appendMessage(f14, 1, f14_1)
	b = appendMessage(b, 14, f14)

	b = appendEmptyMessage(b, 17)

	var f18 []byte
	f18 = appendEmptyMessage(f18, 1)
	var f18_2 []byte
	f18_2 = appendEmptyMessage(f18_2, 1)
	f18 = appendMessage(f18, 2, f18_2)
	b = appendMessage(b, 18, f18)

	var f20 []byte
	var f20_2 []byte
	f20_2 = appendEmptyMessage(f20_2, 1)
	f20_2 = appendEmptyMessage(f20_2, 2)
	f20 = appendMessage(f20, 2, f20_2)
	b = appendMessage(b, 20, f20)

	b = appendEmptyMessage(b, 22)
	b = appendEmptyMessage(b, 23)
	b = appendEmptyMessage(b, 24)

	return b
}

// buildField1_3 builds the field 1.3 structure
func buildField1_3() []byte {
	var b []byte
	b = appendEmptyMessage(b, 2)

	var f3 []byte
	f3 = appendEmptyMessage(f3, 2)
	f3 = appendEmptyMessage(f3, 3)
	f3 = appendEmptyMessage(f3, 7)
	f3 = appendEmptyMessage(f3, 8)
	var f3_14 []byte
	f3_14 = appendEmptyMessage(f3_14, 1)
	f3 = appendMessage(f3, 14, f3_14)
	f3 = appendEmptyMessage(f3, 16)
	var f3_17 []byte
	f3_17 = appendEmptyMessage(f3_17, 2)
	f3 = appendMessage(f3, 17, f3_17)
	f3 = appendEmptyMessage(f3, 18)
	f3 = appendEmptyMessage(f3, 19)
	f3 = appendEmptyMessage(f3, 20)
	f3 = appendEmptyMessage(f3, 21)
	f3 = appendEmptyMessage(f3, 22)
	f3 = appendEmptyMessage(f3, 23)
	var f3_27 []byte
	f3_27 = appendEmptyMessage(f3_27, 1)
	var f3_27_2 []byte
	f3_27_2 = appendEmptyMessage(f3_27_2, 1)
	f3_27 = appendMessage(f3_27, 2, f3_27_2)
	f3 = appendMessage(f3, 27, f3_27)
	f3 = appendEmptyMessage(f3, 29)
	f3 = appendEmptyMessage(f3, 30)
	f3 = appendEmptyMessage(f3, 31)
	f3 = appendEmptyMessage(f3, 32)
	f3 = appendEmptyMessage(f3, 34)
	f3 = appendEmptyMessage(f3, 37)
	f3 = appendEmptyMessage(f3, 38)
	f3 = appendEmptyMessage(f3, 39)
	f3 = appendEmptyMessage(f3, 41)
	var f3_43 []byte
	f3_43 = appendEmptyMessage(f3_43, 1)
	f3 = appendMessage(f3, 43, f3_43)
	var f3_45 []byte
	var f3_45_1 []byte
	f3_45_1 = appendEmptyMessage(f3_45_1, 1)
	f3_45 = appendMessage(f3_45, 1, f3_45_1)
	f3 = appendMessage(f3, 45, f3_45)
	var f3_46 []byte
	f3_46 = appendEmptyMessage(f3_46, 1)
	f3_46 = appendEmptyMessage(f3_46, 2)
	f3_46 = appendEmptyMessage(f3_46, 3)
	f3 = appendMessage(f3, 46, f3_46)
	f3 = appendEmptyMessage(f3, 47)
	b = appendMessage(b, 3, f3)

	var f4 []byte
	f4 = appendEmptyMessage(f4, 2)
	var f4_3 []byte
	f4_3 = appendEmptyMessage(f4_3, 1)
	f4 = appendMessage(f4, 3, f4_3)
	f4 = appendEmptyMessage(f4, 4)
	var f4_5 []byte
	f4_5 = appendEmptyMessage(f4_5, 1)
	f4 = appendMessage(f4, 5, f4_5)
	b = appendMessage(b, 4, f4)

	b = appendEmptyMessage(b, 7)
	b = appendEmptyMessage(b, 12)
	b = appendEmptyMessage(b, 13)

	var f14 []byte
	f14 = appendEmptyMessage(f14, 1)
	var f14_2 []byte
	f14_2 = appendEmptyMessage(f14_2, 1)
	var f14_2_2 []byte
	f14_2_2 = appendEmptyMessage(f14_2_2, 1)
	f14_2 = appendMessage(f14_2, 2, f14_2_2)
	f14_2 = appendEmptyMessage(f14_2, 3)
	var f14_2_4 []byte
	f14_2_4 = appendEmptyMessage(f14_2_4, 1)
	f14_2 = appendMessage(f14_2, 4, f14_2_4)
	f14 = appendMessage(f14, 2, f14_2)
	var f14_3 []byte
	f14_3 = appendEmptyMessage(f14_3, 1)
	var f14_3_2 []byte
	f14_3_2 = appendEmptyMessage(f14_3_2, 1)
	f14_3 = appendMessage(f14_3, 2, f14_3_2)
	f14_3 = appendEmptyMessage(f14_3, 3)
	f14_3 = appendEmptyMessage(f14_3, 4)
	f14 = appendMessage(f14, 3, f14_3)
	b = appendMessage(b, 14, f14)

	b = appendEmptyMessage(b, 15)

	var f16 []byte
	f16 = appendEmptyMessage(f16, 1)
	b = appendMessage(b, 16, f16)

	b = appendEmptyMessage(b, 18)

	var f19 []byte
	var f19_4 []byte
	f19_4 = appendEmptyMessage(f19_4, 2)
	f19 = appendMessage(f19, 4, f19_4)
	var f19_6 []byte
	f19_6 = appendEmptyMessage(f19_6, 2)
	f19_6 = appendEmptyMessage(f19_6, 3)
	f19 = appendMessage(f19, 6, f19_6)
	var f19_7 []byte
	f19_7 = appendEmptyMessage(f19_7, 2)
	f19_7 = appendEmptyMessage(f19_7, 3)
	f19 = appendMessage(f19, 7, f19_7)
	f19 = appendEmptyMessage(f19, 8)
	f19 = appendEmptyMessage(f19, 9)
	b = appendMessage(b, 19, f19)

	b = appendEmptyMessage(b, 20)
	b = appendEmptyMessage(b, 22)
	b = appendEmptyMessage(b, 24)
	b = appendEmptyMessage(b, 25)
	b = appendEmptyMessage(b, 26)

	return b
}

// buildField1_9 builds the field 1.9 structure
func buildField1_9() []byte {
	var b []byte

	var f1 []byte
	var f1_2 []byte
	f1_2 = appendEmptyMessage(f1_2, 1)
	f1_2 = appendEmptyMessage(f1_2, 2)
	f1 = appendMessage(f1, 2, f1_2)
	b = appendMessage(b, 1, f1)

	var f2 []byte
	var f2_3 []byte
	f2_3 = appendIntField(f2_3, 2, 1)
	f2 = appendMessage(f2, 3, f2_3)
	b = appendMessage(b, 2, f2)

	var f3 []byte
	f3 = appendEmptyMessage(f3, 2)
	b = appendMessage(b, 3, f3)

	b = appendEmptyMessage(b, 4)

	var f7 []byte
	f7 = appendEmptyMessage(f7, 1)
	b = appendMessage(b, 7, f7)

	var f8 []byte
	f8 = appendIntField(f8, 1, 2)
	f8 = appendStringField(f8, 2, "\x01\x02\x03\x05\x06\x07")
	b = appendMessage(b, 8, f8)

	b = appendEmptyMessage(b, 9)

	var f11 []byte
	f11 = appendEmptyMessage(f11, 1)
	b = appendMessage(b, 11, f11)

	return b
}

// buildField1_12 builds the field 1.12 structure
func buildField1_12() []byte {
	var b []byte
	var f2 []byte
	f2 = appendEmptyMessage(f2, 1)
	f2 = appendEmptyMessage(f2, 2)
	b = appendMessage(b, 2, f2)
	var f3 []byte
	f3 = appendEmptyMessage(f3, 1)
	b = appendMessage(b, 3, f3)
	b = appendEmptyMessage(b, 4)
	return b
}

// buildField1_15 builds the field 1.15 structure
func buildField1_15() []byte {
	var b []byte
	var f3 []byte
	f3 = appendIntField(f3, 1, 1)
	b = appendMessage(b, 3, f3)
	return b
}

// buildField1_19 builds the field 1.19 structure
func buildField1_19() []byte {
	var b []byte

	var f1 []byte
	f1 = appendEmptyMessage(f1, 1)
	f1 = appendEmptyMessage(f1, 2)
	b = appendMessage(b, 1, f1)

	var f2 []byte
	f2 = appendIntField(f2, 1, 1)
	f2 = appendIntField(f2, 1, 2)
	f2 = appendIntField(f2, 1, 4)
	f2 = appendIntField(f2, 1, 6)
	f2 = appendIntField(f2, 1, 5)
	f2 = appendIntField(f2, 1, 7)
	b = appendMessage(b, 2, f2)

	var f3 []byte
	f3 = appendEmptyMessage(f3, 1)
	f3 = appendEmptyMessage(f3, 2)
	b = appendMessage(b, 3, f3)

	var f5 []byte
	f5 = appendEmptyMessage(f5, 1)
	f5 = appendEmptyMessage(f5, 2)
	b = appendMessage(b, 5, f5)

	var f6 []byte
	f6 = appendEmptyMessage(f6, 1)
	b = appendMessage(b, 6, f6)

	var f7 []byte
	f7 = appendEmptyMessage(f7, 1)
	f7 = appendEmptyMessage(f7, 2)
	b = appendMessage(b, 7, f7)

	var f8 []byte
	f8 = appendEmptyMessage(f8, 1)
	b = appendMessage(b, 8, f8)

	return b
}

// buildField1_21 builds the field 1.21 structure
func buildField1_21() []byte {
	var b []byte

	var f2 []byte
	var f2_2 []byte
	f2_2 = appendEmptyMessage(f2_2, 4)
	f2 = appendMessage(f2, 2, f2_2)
	f2 = appendEmptyMessage(f2, 4)
	f2 = appendEmptyMessage(f2, 5)
	b = appendMessage(b, 2, f2)

	var f3 []byte
	var f3_2 []byte
	f3_2 = appendIntField(f3_2, 1, 1)
	f3 = appendMessage(f3, 2, f3_2)
	var f3_4 []byte
	f3_4 = appendEmptyMessage(f3_4, 2)
	f3 = appendMessage(f3, 4, f3_4)
	b = appendMessage(b, 3, f3)

	var f5 []byte
	f5 = appendEmptyMessage(f5, 1)
	b = appendMessage(b, 5, f5)

	var f6 []byte
	f6 = appendEmptyMessage(f6, 1)
	var f6_2 []byte
	f6_2 = appendEmptyMessage(f6_2, 1)
	f6 = appendMessage(f6, 2, f6_2)
	b = appendMessage(b, 6, f6)

	var f7 []byte
	f7 = appendIntField(f7, 1, 2)
	f7 = appendStringField(f7, 2, "\x01\x07\x08\t\n\r\x0e\x0f\x11\x13\x14\x16\x17-./01:\x06\x18267;>?@A89<GBED")
	f7 = appendStringField(f7, 3, "\x01")
	b = appendMessage(b, 7, f7)

	var f8 []byte
	var f8_3 []byte
	var f8_3_1 []byte
	var f8_3_1_1 []byte
	var f8_3_1_1_2 []byte
	f8_3_1_1_2 = appendIntField(f8_3_1_1_2, 1, 1)
	f8_3_1_1 = appendMessage(f8_3_1_1, 2, f8_3_1_1_2)
	var f8_3_1_1_4 []byte
	f8_3_1_1_4 = appendEmptyMessage(f8_3_1_1_4, 2)
	f8_3_1_1 = appendMessage(f8_3_1_1, 4, f8_3_1_1_4)
	f8_3_1 = appendMessage(f8_3_1, 1, f8_3_1_1)
	f8_3 = appendMessage(f8_3, 1, f8_3_1)
	f8_3 = appendEmptyMessage(f8_3, 3)
	f8 = appendMessage(f8, 3, f8_3)
	var f8_4 []byte
	f8_4 = appendEmptyMessage(f8_4, 1)
	f8 = appendMessage(f8, 4, f8_4)
	var f8_5 []byte
	var f8_5_1 []byte
	var f8_5_1_2 []byte
	f8_5_1_2 = appendIntField(f8_5_1_2, 1, 1)
	f8_5_1 = appendMessage(f8_5_1, 2, f8_5_1_2)
	var f8_5_1_4 []byte
	f8_5_1_4 = appendEmptyMessage(f8_5_1_4, 2)
	f8_5_1 = appendMessage(f8_5_1, 4, f8_5_1_4)
	f8_5 = appendMessage(f8_5, 1, f8_5_1)
	f8 = appendMessage(f8, 5, f8_5)
	b = appendMessage(b, 8, f8)

	var f9 []byte
	f9 = appendEmptyMessage(f9, 1)
	b = appendMessage(b, 9, f9)

	var f10 []byte
	var f10_1 []byte
	f10_1 = appendEmptyMessage(f10_1, 1)
	f10 = appendMessage(f10, 1, f10_1)
	f10 = appendEmptyMessage(f10, 3)
	f10 = appendEmptyMessage(f10, 5)
	var f10_6 []byte
	f10_6 = appendEmptyMessage(f10_6, 1)
	f10 = appendMessage(f10, 6, f10_6)
	f10 = appendEmptyMessage(f10, 7)
	f10 = appendEmptyMessage(f10, 9)
	f10 = appendEmptyMessage(f10, 10)
	b = appendMessage(b, 10, f10)

	b = appendEmptyMessage(b, 11)
	b = appendEmptyMessage(b, 12)
	b = appendEmptyMessage(b, 13)
	b = appendEmptyMessage(b, 14)

	var f16 []byte
	f16 = appendEmptyMessage(f16, 1)
	b = appendMessage(b, 16, f16)

	return b
}

// buildField1_22 builds the field 1.22 structure
func buildField1_22() []byte {
	var b []byte
	b = appendIntField(b, 1, 1)
	b = appendStringField(b, 2, "107818234414673686888")
	return b
}

// buildField1_25 builds the field 1.25 structure
func buildField1_25() []byte {
	var b []byte
	var f1 []byte
	var f1_1 []byte
	var f1_1_1 []byte
	f1_1_1 = appendEmptyMessage(f1_1_1, 1)
	f1_1 = appendMessage(f1_1, 1, f1_1_1)
	f1 = appendMessage(f1, 1, f1_1)
	b = appendMessage(b, 1, f1)
	b = appendEmptyMessage(b, 2)
	return b
}

// buildField2 builds the field 2 structure
func buildField2() []byte {
	var b []byte
	var f1 []byte
	var f1_1 []byte
	var f1_1_1 []byte
	f1_1_1 = appendEmptyMessage(f1_1_1, 1)
	f1_1 = appendMessage(f1_1, 1, f1_1_1)
	f1_1 = appendEmptyMessage(f1_1, 2)
	f1 = appendMessage(f1, 1, f1_1)
	b = appendMessage(b, 1, f1)
	b = appendEmptyMessage(b, 2)
	return b
}

// buildLibraryStateRequest builds the complete request for get_library_state
func buildLibraryStateRequest(stateToken, pageToken string) []byte {
	var field1 []byte

	field1 = appendMessage(field1, 1, buildField1_1())
	field1 = appendMessage(field1, 2, buildField1_2())
	field1 = appendMessage(field1, 3, buildField1_3())

	if pageToken != "" {
		field1 = appendStringField(field1, 4, pageToken)
	}

	if stateToken != "" {
		field1 = appendStringField(field1, 6, stateToken)
	}

	field1 = appendIntField(field1, 7, 2)
	field1 = appendMessage(field1, 9, buildField1_9())

	field1 = appendIntField(field1, 11, 1)
	field1 = appendIntField(field1, 11, 2)
	field1 = appendIntField(field1, 11, 6)

	field1 = appendMessage(field1, 12, buildField1_12())
	field1 = appendEmptyMessage(field1, 13)
	field1 = appendMessage(field1, 15, buildField1_15())
	field1 = appendMessage(field1, 19, buildField1_19())
	field1 = appendMessage(field1, 21, buildField1_21())
	field1 = appendMessage(field1, 22, buildField1_22())
	field1 = appendMessage(field1, 25, buildField1_25())
	field1 = appendEmptyMessage(field1, 26)

	var result []byte
	result = appendMessage(result, 1, field1)
	result = appendMessage(result, 2, buildField2())

	return result
}

// buildLibraryPageInitRequest builds the request for get_library_page_init
func buildLibraryPageInitRequest(pageToken string) []byte {
	return buildLibraryStateRequest("", pageToken)
}

// buildLibraryPageRequest builds the request for get_library_page
func buildLibraryPageRequest(pageToken, stateToken string) []byte {
	return buildLibraryStateRequest(stateToken, pageToken)
}
