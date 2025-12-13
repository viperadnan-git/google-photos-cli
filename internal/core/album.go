package core

import (
	"fmt"
	"time"

	"github.com/viperadnan-git/go-gpm/internal/pb"
)

// CreateAlbum creates a new album with optional media keys
func (a *Api) CreateAlbum(albumName string, mediaKeys []string) (string, error) {
	var mediaKeyRefs []*pb.CreateAlbum_MediaKeyRef
	var field3Value int64

	if len(mediaKeys) > 0 {
		// When creating album with media, Field3 = 1
		field3Value = 1
		mediaKeyRefs = make([]*pb.CreateAlbum_MediaKeyRef, len(mediaKeys))
		for i, key := range mediaKeys {
			mediaKeyRefs[i] = &pb.CreateAlbum_MediaKeyRef{
				Field1: &pb.CreateAlbum_MediaKeyRef_MediaKeyInner{
					MediaKey: key,
				},
			}
		}
	} else {
		// When creating album without media, Field3 = 2
		field3Value = 2
	}

	requestBody := pb.CreateAlbum{
		AlbumName: albumName,
		Timestamp: time.Now().Unix(),
		Field3:    field3Value,
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

// DeleteAlbum deletes an album by its media key
func (a *Api) DeleteAlbum(albumMediaKey string) error {
	requestBody := pb.DeleteAlbum{
		AlbumMediaKey: albumMediaKey,
	}

	return a.DoProtoRequest(
		"https://photosdata-pa.googleapis.com/6439526531001121323/11165707358190966680",
		&requestBody,
		nil,
		WithAuth(),
		WithCommonHeaders(),
		WithStatusCheck(),
	)
}

// RenameAlbum renames an album
func (a *Api) RenameAlbum(albumMediaKey string, newName string) error {
	requestBody := pb.RenameAlbum{
		AlbumMediaKey: albumMediaKey,
		NewName:       newName,
		Field3:        1,
	}

	return a.DoProtoRequest(
		"https://photosdata-pa.googleapis.com/6439526531001121323/16466587394238175348",
		&requestBody,
		nil,
		WithAuth(),
		WithCommonHeaders(),
		WithStatusCheck(),
	)
}
