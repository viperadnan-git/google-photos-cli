# Google Photos Library State API Implementation

This document details the reverse-engineered Google Photos library state API used for syncing media items and albums.

## API Overview

### Endpoint
```
POST https://photosdata-pa.googleapis.com/6439526531001121323/18047484249733410717
```

### Required Headers
```
Accept-Encoding: gzip
Accept-Language: <language>
Content-Type: application/x-protobuf
Authorization: Bearer <token>
x-goog-ext-173412678-bin: CgcIAhClARgC
x-goog-ext-174067345-bin: CgIIAg==
```

### Three API Variants
All use the same endpoint with different request payloads:

| Function | Purpose | Tokens Used |
|----------|---------|-------------|
| `get_library_state` | Initial sync or incremental updates | `state_token` (optional) |
| `get_library_page_init` | Paginated fetch (first call) | `page_token` |
| `get_library_page` | Paginated fetch (subsequent) | `page_token` + `state_token` |

---

## Token System & API Usage

### Understanding Tokens

**`state_token`** - Represents library state at a point in time
- Returned in every response
- Used for incremental sync (get only changes since this state)
- Encodes: timestamp, sync position, library version
- Long-lived: can be stored and reused later

**`page_token`** - Pagination cursor within a single sync operation
- Returned when more items exist (`next_page_token`)
- Used to fetch subsequent pages
- Short-lived: valid only during current sync session
- Becomes invalid after library changes

**`next_page_token`** - Same as `page_token`, returned in response when more pages available

### API Function Details

#### 1. `get_library_state(state_token?)`

**Purpose:** Primary sync function for fetching library contents.

**Behavior:**
- Without `state_token`: Returns ALL items (full sync)
- With `state_token`: Returns only CHANGES since that state (incremental sync)

**Use cases:**
```
Initial sync:      get_library_state()           → all items + state_token
Incremental sync:  get_library_state(old_token)  → changes + new state_token
```

**Response includes:**
- `media_items`: New or modified items
- `albums`: New or modified albums
- `deletions`: Items deleted since state_token
- `state_token`: New state for next sync
- `next_page_token`: If more pages exist

#### 2. `get_library_page_init(page_token)`

**Purpose:** Start paginated fetch from a specific position.

**When to use:**
- Resuming an interrupted sync
- Starting from a known position
- First page of a paginated operation

**Behavior:**
- Uses `page_token` to determine starting position
- Returns first page of results from that position
- Returns both `state_token` and `next_page_token`

```
Resume sync: get_library_page_init(saved_page_token) → results + state_token
```

#### 3. `get_library_page(page_token, state_token)`

**Purpose:** Fetch subsequent pages during pagination.

**When to use:**
- After `get_library_state()` returns `next_page_token`
- After `get_library_page_init()` returns `next_page_token`
- Continuing through large result sets

**Behavior:**
- Combines position (`page_token`) with state (`state_token`)
- Ensures consistency across pages
- Prevents missing/duplicate items if library changes mid-sync

```
Page 2+: get_library_page(next_page_token, state_token) → more results
```

### Usage Patterns

#### Pattern 1: Full Sync (Small Library)
```
1. resp = get_library_state()
2. process(resp.media_items, resp.albums)
3. while resp.next_page_token:
       resp = get_library_page(resp.next_page_token, resp.state_token)
       process(resp.media_items, resp.albums)
4. store(resp.state_token)  // for future incremental sync
```

#### Pattern 2: Incremental Sync
```
1. resp = get_library_state(stored_state_token)
2. process_changes(resp.media_items)
3. process_deletions(resp.deletions)
4. while resp.next_page_token:
       resp = get_library_page(resp.next_page_token, resp.state_token)
       process_changes(resp.media_items)
5. store(resp.state_token)  // update for next sync
```

#### Pattern 3: Resumable Sync
```
// Initial or interrupted sync
1. resp = get_library_state() or get_library_page_init(saved_page_token)
2. process(resp)
3. store(resp.next_page_token, resp.state_token)  // checkpoint

// On interrupt/crash, resume:
4. resp = get_library_page_init(saved_page_token)
5. continue from step 2
```

### Token Lifecycle Diagram
```
┌─────────────────────────────────────────────────────────────────┐
│                        FULL SYNC                                │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  get_library_state()                                            │
│         │                                                       │
│         ▼                                                       │
│  ┌──────────────────┐                                           │
│  │ Response Page 1  │                                           │
│  │ - media_items    │                                           │
│  │ - state_token ●──┼─────────────────────────┐                 │
│  │ - next_page_token│                         │                 │
│  └────────┬─────────┘                         │                 │
│           │                                   │                 │
│           ▼                                   ▼                 │
│  get_library_page(next_page_token, state_token)                 │
│           │                                                     │
│           ▼                                                     │
│  ┌──────────────────┐                                           │
│  │ Response Page 2  │                                           │
│  │ - media_items    │                                           │
│  │ - state_token ●──┼─────────────────────────┐                 │
│  │ - next_page_token│ (null if last page)     │                 │
│  └──────────────────┘                         │                 │
│                                               ▼                 │
│                                    STORE state_token            │
│                                    for incremental sync         │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                     INCREMENTAL SYNC                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  get_library_state(stored_state_token)                          │
│         │                                                       │
│         ▼                                                       │
│  ┌──────────────────┐                                           │
│  │ Response         │                                           │
│  │ - media_items    │  ← Only NEW/MODIFIED since state_token    │
│  │ - deletions      │  ← Items DELETED since state_token        │
│  │ - state_token ●──┼──► STORE for next incremental sync        │
│  └──────────────────┘                                           │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Response Schema

### Main Structure
Reference: [`.proto/LibraryState.proto`](../.proto/LibraryState.proto)

```
LibraryStateResponse
├── data (field 1): LibraryStateData
│   ├── next_page_token (field 1): string
│   ├── media_items (field 2): repeated MediaItemData
│   ├── albums (field 3): repeated AlbumData
│   ├── state_token (field 6): string
│   └── deletions (field 9): repeated DeletionData
└── metadata (field 2): not parsed
```

### Media Item Structure
```
MediaItemData (field 2 in LibraryStateData)
├── media_key (field 1): string - Unique identifier
├── metadata (field 2): MediaMetadata
│   ├── collection.collection_id (field 1.1): string - Album ID if in album
│   ├── file_name (field 4): string
│   ├── properties (field 5): repeated PropertyInfo
│   │   └── property_id (field 1): uint64 - 27 = non-canonical marker
│   ├── utc_timestamp (field 7): uint64 - Capture time (ms)
│   ├── timezone_offset (field 8): uint64 - TZ offset (ms)
│   ├── server_creation_timestamp (field 9): uint64 - Upload time (ms)
│   ├── size_bytes (field 10): uint64
│   ├── upload_status (field 11): uint64
│   ├── dedup.dedup_key (field 21.1): string - Links duplicates
│   ├── content_version (field 26): uint64
│   ├── archive.status (field 29.1): uint64 - 1=archived, 2=not
│   ├── favorite.status (field 31.1): uint64 - 1=favorite, 2=not
│   ├── quota.quota_charged_bytes (field 35.2): uint64
│   ├── quota.quality_type (field 35.3): uint64 - 2=original
│   └── lock.status (field 39.1): uint64 - 1=locked
├── type_info (field 5): MediaTypeInfo
│   ├── media_type (field 1): uint64 - 1=photo, 2=video
│   ├── photo (field 2): PhotoInfo
│   │   └── details (field 1): PhotoDetails
│   │       ├── thumbnail_url (field 1): string
│   │       └── dimensions (field 9): PhotoDimensions
│   │           ├── width (field 1): uint64
│   │           └── height (field 2): uint64
│   └── video (field 3): VideoInfo
│       ├── thumbnail (field 2): VideoThumbnail
│       │   └── url (field 1): string
│       ├── details (field 4): VideoDetails
│       │   ├── duration_ms (field 1): uint64
│       │   ├── width (field 4): uint64
│       │   └── height (field 5): uint64
│       └── download_url (field 5): string
└── key_info (field 6): MediaKeyInfo - Alternative key
    └── media_key (field 1): string
```

### Album Structure
```
AlbumData (field 3 in LibraryStateData)
├── album_key (field 1): string - Unique identifier
├── metadata (field 2): AlbumMetadata
│   ├── title (field 5): string
│   ├── album_type (field 8): uint64 - 1=user album
│   └── cover_item (field 17): CoverItemInfo
│       └── media_key (field 1): string
└── sort_info (field 19): AlbumSortInfo
    ├── sort_order (field 1): uint64
    └── is_custom_ordered (field 2): uint64 - 1=custom
```

**Note:** Album `item_count` (field 2.7) always returns 0 from server. Must be calculated by counting media items with matching `collection_id`.

### Deletion Structure
```
DeletionData (field 9 in LibraryStateData)
└── info (field 1): DeletionInfo
    ├── deletion_type (field 1): uint64 - 1=media, 4=collection, 6=other
    ├── media (field 2): DeletedMediaInfo (when type=1)
    │   └── media_key (field 1): string
    ├── collection (field 5): when type=4
    └── other (field 7): when type=6
```

---

## Key Concepts

### Canonical vs Non-Canonical Items

**Property 27** in `metadata.properties` marks non-canonical items:
- **Canonical (original)**: No property 27 - the source item in library
- **Non-canonical (copy)**: Has property 27 - a reference in an album

Items with the same `dedup_key` are the same media (original + copies).

```
Original Item (canonical)          Album Copy (non-canonical)
├── media_key: "ABC123"            ├── media_key: "XYZ789"
├── dedup_key: "HASH1"             ├── dedup_key: "HASH1" (same)
├── album_id: ""                   ├── album_id: "ALBUM1"
└── properties: []                 └── properties: [{id: 27}]
```

### Dedup Key
- SHA1-based hash identifying unique content
- Same `dedup_key` = same file (regardless of media_key)
- Used to link album copies to originals
- Format: URL-safe base64 encoded

### Album ID / Collection ID
- `collection_id` in media item metadata = `album_key` of containing album
- Canonical items may have `album_id` (created directly in album)
- Non-canonical items always have `album_id` (the album they're shared to)

---

## Request Building

### Request Structure (Wire Format)
```
Request
├── field 1: Main body
│   ├── field 1: Config block 1.1 (projection config)
│   ├── field 2: Config block 1.2 (album config)
│   ├── field 3: Config block 1.3 (deletion config)
│   ├── field 4: page_token (string, optional)
│   ├── field 6: state_token (string, optional)
│   ├── field 7: constant 2
│   ├── field 9: Config block 1.9 (sync config)
│   ├── field 11: repeated [1, 2, 6] (item types)
│   ├── field 12-26: Various config blocks
│   └── ...
└── field 2: Metadata block
```

### Config Blocks Purpose
| Block | Purpose |
|-------|---------|
| 1.1 | Media item field projection (which fields to return) |
| 1.2 | Album field projection |
| 1.3 | Deletion tracking config |
| 1.9 | Sync behavior config |
| 1.11 | Item types to include (1=media, 2=albums, 6=deletions) |
| 1.19 | Category/collection type filters |
| 1.21 | Advanced filters and sort options |

Reference implementation: [`internal/core/library.go`](../internal/core/library.go) - `buildField1_*` functions

---

## Parsed Output Structure

### Go Types
Reference: [`internal/core/library.go`](../internal/core/library.go)

```go
type LibraryStateResponse struct {
    StateToken       string      // For incremental sync
    NextPageToken    string      // For pagination
    MediaItems       []MediaItem // Only canonical items
    Albums           []AlbumItem
    DeletedMediaKeys []string
}

type MediaItem struct {
    MediaKey           string
    FileName           string
    DedupKey           string          // Links duplicates
    Type               int             // 1=photo, 2=video
    SizeBytes          uint64
    UTCTimestamp       uint64          // Capture time (ms)
    ServerCreationTime uint64          // Upload time (ms)
    TimezoneOffset     int64
    ContentVersion     uint64
    Width, Height      int
    ThumbnailURL       string
    DownloadURL        string          // Videos only
    Duration           uint64          // Videos only (ms)
    IsArchived         bool
    IsFavorite         bool
    IsLocked           bool
    IsOriginalQuality  bool
    QuotaChargedBytes  uint64
    AlbumID            string          // If created in album
    Copies             []MediaItemCopy // Non-canonical copies
}

type MediaItemCopy struct {
    MediaKey string // Copy's key in album
    AlbumID  string // Album containing copy
}

type AlbumItem struct {
    AlbumKey      string
    Title         string
    ItemCount     int              // Calculated from media items
    Type          int              // 1=user album
    CoverMediaKey string
    MediaItems    []AlbumMediaItem // Items in this album
}

type AlbumMediaItem struct {
    MediaKey         string // Copy key in album
    OriginalMediaKey string // Canonical item key
    FileName         string
    DedupKey         string
}
```

---

## Sync Strategies

### Full Sync
1. Call `get_library_state()` with no tokens
2. Iterate with `next_page_token` until exhausted
3. Store final `state_token` for incremental sync

### Incremental Sync
1. Call `get_library_state(state_token)` with stored token
2. Response contains only changes since last sync
3. `deletions` array contains removed items
4. Update stored `state_token`

### Pagination
```
get_library_page_init(page_token) → first page + state_token
get_library_page(page_token, state_token) → subsequent pages
```

---

## Field Value Reference

### Media Type (field 5.1)
| Value | Type |
|-------|------|
| 1 | Photo |
| 2 | Video |

### Archive Status (field 2.29.1)
| Value | Status |
|-------|--------|
| 1 | Archived |
| 2 | Not archived |

### Favorite Status (field 2.31.1)
| Value | Status |
|-------|--------|
| 1 | Favorite |
| 2 | Not favorite |

### Quality Type (field 2.35.3)
| Value | Quality |
|-------|---------|
| 2 | Original quality |
| Other | Storage saver |

### Album Type (field 2.8)
| Value | Type |
|-------|------|
| 1 | User-created album |

### Deletion Type (field 9.1.1)
| Value | Type |
|-------|------|
| 1 | Media item |
| 4 | Collection/Album |
| 6 | Other |

---

## Smart Collections (Not Implemented)

Auto-generated collections like Screenshots, Videos, etc. Available at field 10 in LibraryStateData.

```
SmartCollectionData
├── key (field 1): SmartCollectionKey
│   └── collection_key (field 1): string
├── metadata (field 2): SmartCollectionMetadata
│   ├── name (field 6): string
│   └── cover (field 7): SmartCollectionCover
└── collection_type (field 3): uint64 - 6=Videos, 7=category
```

Kept as comments in proto file for future use.

---

## Implementation Files

| File | Purpose |
|------|---------|
| `.proto/LibraryState.proto` | Protobuf schema definitions |
| `internal/pb/LibraryState.pb.go` | Generated Go protobuf code |
| `internal/core/library.go` | API methods and parsing logic |
| `main.go` | Public type exports |
| `cmd/gpcli/library.go` | CLI test command |
