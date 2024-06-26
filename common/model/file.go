package model

import (
	"dht-ocean/common/bittorrent"
	"dht-ocean/ocean/ocean"
	"encoding/hex"
)

type File struct {
	Length   int64    `bson:"length,omitempty"`
	Paths    []string `bson:"paths,omitempty"`
	FileHash string   `bson:"file_hash,omitempty"`
}

func NewFileFromProto(p *ocean.File) *File {
	return &File{
		Length:   p.Length,
		Paths:    p.Paths,
		FileHash: hex.EncodeToString(p.FileHash),
	}
}

func NewFileFromBTFile(p *bittorrent.File) *File {
	return &File{
		Length:   p.Length,
		Paths:    p.Path,
		FileHash: hex.EncodeToString(p.FileHash),
	}
}
