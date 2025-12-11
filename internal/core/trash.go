package core

import (
	"strconv"

	"github.com/viperadnan-git/gogpm/internal/pb"
)

// MoveToTrash moves items to trash
// itemKeys can be either mediaKeys or dedupKeys (URL-safe base64 encoded SHA1 hashes)
func (a *Api) MoveToTrash(itemKeys []string) error {
	requestBody := pb.MoveToTrash{
		Field2:   1,
		ItemKeys: itemKeys,
		Field4:   1,
		Field8: &pb.MoveToTrash_Field8{
			Field4: &pb.MoveToTrash_Field8_Field4{
				Field2: &pb.MoveToTrash_Field8_Field4_Empty{},
				Field3: &pb.MoveToTrash_Field8_Field4_Field3{
					Field1: &pb.MoveToTrash_Field8_Field4_Empty{},
				},
				Field4: &pb.MoveToTrash_Field8_Field4_Empty{},
				Field5: &pb.MoveToTrash_Field8_Field4_Field5{
					Field1: &pb.MoveToTrash_Field8_Field4_Empty{},
				},
			},
		},
		Field9: &pb.MoveToTrash_Field9{
			Field1: 5,
			Field2: &pb.MoveToTrash_Field9_Field2{
				Field1: a.ClientVersionCode,
				Field2: strconv.FormatInt(a.AndroidAPIVersion, 10),
			},
		},
	}

	return a.DoProtoRequest(
		"https://photosdata-pa.googleapis.com/6439526531001121323/17490284929287180316",
		&requestBody,
		nil,
		WithAuth(),
		WithCommonHeaders(),
		WithStatusCheck(),
	)
}

// RestoreFromTrash restores items from trash
// itemKeys can be either mediaKeys or dedupKeys (URL-safe base64 encoded SHA1 hashes)
func (a *Api) RestoreFromTrash(itemKeys []string) error {
	requestBody := pb.RestoreFromTrash{
		Field2:   3,
		ItemKeys: itemKeys,
		Field4:   2,
		Field8: &pb.RestoreFromTrash_Field8{
			Field4: &pb.RestoreFromTrash_Field8_Field4{
				Field2: &pb.RestoreFromTrash_Field8_Field4_Empty{},
				Field3: &pb.RestoreFromTrash_Field8_Field4_Field3{
					Field1: &pb.RestoreFromTrash_Field8_Field4_Empty{},
				},
			},
		},
		Field9: &pb.RestoreFromTrash_Field9{
			Field1: 5,
			Field2: &pb.RestoreFromTrash_Field9_Field2{
				Field1: a.ClientVersionCode,
				Field2: strconv.FormatInt(a.AndroidAPIVersion, 10),
			},
		},
	}

	return a.DoProtoRequest(
		"https://photosdata-pa.googleapis.com/6439526531001121323/17490284929287180316",
		&requestBody,
		nil,
		WithAuth(),
		WithCommonHeaders(),
		WithStatusCheck(),
	)
}
