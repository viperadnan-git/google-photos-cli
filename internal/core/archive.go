package core

import (
	"github.com/viperadnan-git/gogpm/internal/pb"
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

	return a.DoProtoRequest(
		"https://photosdata-pa.googleapis.com/6439526531001121323/6715446385130606868",
		&requestBody,
		nil,
		WithAuth(),
		WithCommonHeaders(),
		WithStatusCheck(),
	)
}
