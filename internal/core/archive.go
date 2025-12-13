package core

import (
	"github.com/viperadnan-git/go-gpm/internal/pb"
)

// SetArchived sets or removes the archived status for multiple items
// itemKeys can be either mediaKeys or dedupKeys (URL-safe base64 encoded SHA1 hashes)
// isArchived: true = archive, false = unarchive
func (a *Api) SetArchived(itemKeys []string, isArchived bool) error {
	var actionType pb.ArchiveActionType
	if isArchived {
		actionType = pb.ArchiveActionType_ARCHIVE
	} else {
		actionType = pb.ArchiveActionType_UNARCHIVE
	}

	items := make([]*pb.ArchiveItems_Item, len(itemKeys))
	for i, key := range itemKeys {
		items[i] = &pb.ArchiveItems_Item{
			ItemKey: key,
			Action: &pb.ArchiveItems_Action{
				Action: actionType,
			},
		}
	}

	requestBody := pb.ArchiveItems{
		Items:  items,
		Field3: 1,
	}

	return a.DoProtoRequest(
		"https://photosdata-pa.googleapis.com/6439526531001121323/6715446385130606868",
		&requestBody,
		nil,
		WithAuth(),
		WithCommonHeaders(),
		WithStatusCheck(),
	)
}
