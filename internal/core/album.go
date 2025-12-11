package core

import (
	"fmt"
	"time"

	"github.com/viperadnan-git/gogpm/internal/pb"
)

// CreateAlbum creates a new album with the specified media keys
func (a *Api) CreateAlbum(albumName string, mediaKeys []string) (string, error) {
	mediaKeyRefs := make([]*pb.CreateAlbum_MediaKeyRef, len(mediaKeys))
	for i, key := range mediaKeys {
		mediaKeyRefs[i] = &pb.CreateAlbum_MediaKeyRef{
			Field1: &pb.CreateAlbum_MediaKeyRef_MediaKeyInner{
				MediaKey: key,
			},
		}
	}

	requestBody := pb.CreateAlbum{
		AlbumName: albumName,
		Timestamp: time.Now().Unix(),
		Field3:    1,
		MediaKeys: mediaKeyRefs,
		Field6:    &pb.CreateAlbum_Field6Type{},
		Field7:    &pb.CreateAlbum_Field7Type{Field1: 3},
		DeviceInfo: &pb.CreateAlbum_DeviceInfo{
			Model:             a.Model,
			Make:              a.Make,
			AndroidApiVersion: a.AndroidAPIVersion,
		},
	}

	var response pb.CreateAlbumResponse
	if err := a.DoProtoRequest(
		"https://photosdata-pa.googleapis.com/6439526531001121323/8386163679468898444",
		&requestBody,
		&response,
		WithAuth(),
		WithCommonHeaders(),
		WithStatusCheck(),
	); err != nil {
		return "", err
	}

	if response.GetField1() == nil {
		return "", fmt.Errorf("album creation failed: invalid response structure")
	}

	albumMediaKey := response.GetField1().GetAlbumMediaKey()
	if albumMediaKey == "" {
		return "", fmt.Errorf("album creation failed: no album media key returned")
	}

	return albumMediaKey, nil
}

// AddMediaToAlbum adds media items to an existing album
func (a *Api) AddMediaToAlbum(albumMediaKey string, mediaKeys []string) error {
	requestBody := pb.AddMediaToAlbum{
		MediaKeys:     mediaKeys,
		AlbumMediaKey: albumMediaKey,
		Field5:        &pb.AddMediaToAlbum_Field5Type{Field1: 2},
		DeviceInfo: &pb.AddMediaToAlbum_DeviceInfo{
			Model:             a.Model,
			Make:              a.Make,
			AndroidApiVersion: a.AndroidAPIVersion,
		},
		Timestamp: time.Now().Unix(),
	}

	return a.DoProtoRequest(
		"https://photosdata-pa.googleapis.com/6439526531001121323/484917746253879292",
		&requestBody,
		nil,
		WithAuth(),
		WithCommonHeaders(),
		WithStatusCheck(),
	)
}
