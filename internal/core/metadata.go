package core

import (
	"github.com/viperadnan-git/gogpm/internal/pb"
)

// SetCaption sets the caption for a media item
// itemKey can be either mediaKey or dedupKey
func (a *Api) SetCaption(itemKey, caption string) error {
	requestBody := pb.SetCaption{
		Caption: caption,
		ItemKey: itemKey,
	}

	return a.DoProtoRequest(
		"https://photosdata-pa.googleapis.com/6439526531001121323/1552790390512470739",
		&requestBody,
		nil,
		WithAuth(),
		WithCommonHeaders(),
		WithStatusCheck(),
	)
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

	return a.DoProtoRequest(
		"https://photosdata-pa.googleapis.com/6439526531001121323/5144645502632292153",
		&requestBody,
		nil,
		WithAuth(),
		WithCommonHeaders(),
		WithStatusCheck(),
	)
}
