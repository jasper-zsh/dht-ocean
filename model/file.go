package model

import "dht-ocean/bittorrent"

type File struct {
	Length int64
	Paths  []string
}

func NewFileFromCrawler(file *bittorrent.File) *File {
	return &File{
		Length: int64(file.Length()),
		Paths:  file.Path(),
	}
}
