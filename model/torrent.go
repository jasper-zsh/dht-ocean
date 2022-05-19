package model

import (
	"dht-ocean/bittorrent"
	"encoding/hex"
	"github.com/kamva/mgm/v3"
	"time"
)

var _ mgm.Model = (*Torrent)(nil)

type Torrent struct {
	InfoHash           string     `bson:"_id" json:"info_hash"`
	Name               string     `bson:"name" json:"name"`
	Files              []*File    `bson:"files" json:"files"`
	Tags               []string   `bson:"tags" json:"tags,omitempty"`
	Type               string     `bson:"type" json:"type,omitempty"`
	Length             int64      `bson:"length" json:"length"`
	CreatedAt          *time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt          *time.Time `bson:"updated_at" json:"updated_at"`
	Seeders            *uint32    `bson:"seeders" json:"seeders,omitempty"`
	Leechers           *uint32    `bson:"leechers" json:"leechers,omitempty"`
	TrackerUpdatedAt   *time.Time `bson:"tracker_updated_at" json:"tracker_updated_at,omitempty"`
	SearchUpdated      bool       `bson:"search_updated" json:"search_updated"`
	TrackerLastTriedAt *time.Time `bson:"tracker_last_tried_at" json:"tracker_last_tried_at,omitempty"`
}

func (t *Torrent) PrepareID(id interface{}) (interface{}, error) {
	return id, nil
}

func (t *Torrent) GetID() interface{} {
	return t.InfoHash
}

func (t *Torrent) SetID(id interface{}) {
	t.InfoHash = id.(string)
}

func NewTorrentFromCrawler(t *bittorrent.Torrent) *Torrent {
	r := &Torrent{
		InfoHash: hex.EncodeToString(t.InfoHash),
		Name:     t.Name(),
	}
	for _, file := range t.Files() {
		r.Length += int64(file.Length())
		r.Files = append(r.Files, NewFileFromCrawler(file))
	}
	return r
}
