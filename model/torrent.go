package model

import (
	"dht-ocean/bittorrent"
	"encoding/hex"
	"github.com/kamva/mgm/v3"
	"time"
)

var _ mgm.Model = (*Torrent)(nil)

type Torrent struct {
	InfoHash           string     `bson:"_id"`
	Name               string     `bson:"name"`
	Files              []*File    `bson:"files"`
	Tags               []string   `bson:"tags"`
	Type               string     `bson:"type"`
	Length             int64      `bson:"length"`
	CreatedAt          time.Time  `bson:"created_at"`
	UpdatedAt          time.Time  `bson:"updated_at"`
	Seeders            *int       `bson:"seeders"`
	Leechers           *int       `bson:"leechers"`
	TrackerUpdatedAt   *time.Time `bson:"tracker_updated_at"`
	SearchUpdated      bool       `bson:"search_updated"`
	TrackerLastTriedAt *time.Time `bson:"tracker_last_tried_at"`
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

func (t *Torrent) Creating() error {
	t.CreatedAt = time.Now().UTC()
	t.SearchUpdated = false
	return nil
}

func (t *Torrent) Saving() error {
	t.UpdatedAt = time.Now().UTC()
	t.SearchUpdated = false
	return nil
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
