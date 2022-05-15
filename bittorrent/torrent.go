package bittorrent

import "dht-ocean/bencode"

type Torrent struct {
	Data     map[string]any
	InfoHash []byte
}

func NewTorrentFromMetadata(infoHash []byte, metadata map[string]any) *Torrent {
	return &Torrent{
		InfoHash: infoHash,
		Data:     metadata,
	}
}

func (t *Torrent) Name() string {
	s, _ := bencode.GetString(t.Data, "name")
	return s
}

func (t *Torrent) HasFiles() bool {
	l, _ := bencode.GetList(t.Data, "files")
	if l == nil {
		return false
	}
	return true
}

func (t *Torrent) Files() []*File {
	l, _ := bencode.GetList(t.Data, "files")
	r := make([]*File, 0, len(l))
	for _, item := range l {
		switch item.(type) {
		case map[string]any:
			r = append(r, NewFileFromMetadata(item.(map[string]any)))
		}
	}
	return r
}

type File struct {
	Data map[string]any
}

func NewFileFromMetadata(metadata map[string]any) *File {
	return &File{
		Data: metadata,
	}
}

func (f *File) Length() int {
	i, _ := bencode.GetInt(f.Data, "length")
	return i
}

func (f *File) Path() []string {
	l, _ := bencode.GetList(f.Data, "path")
	r := make([]string, 0, len(l))
	for _, s := range l {
		r = append(r, string(s.([]byte)))
	}
	return r
}

type TorrentHandler interface {
	HandleTorrent(torrent *Torrent)
}
