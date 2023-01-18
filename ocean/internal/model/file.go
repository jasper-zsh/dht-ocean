package model

import (
	"dht-ocean/ocean/ocean"
	"encoding/hex"
)

type File struct {
	Length   int64    `bson:"length,omitempty"`
	Path     []string `bson:"path,omitempty"`
	FileHash string   `bson:"file_hash,omitempty"`
}

func NewFileFromProto(p *ocean.File) *File {
	return &File{
		Length:   p.Length,
		Path:     p.Path,
		FileHash: hex.EncodeToString(p.FileHash),
	}
}