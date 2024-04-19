package model

import (
	"dht-ocean/common/bittorrent"
	"dht-ocean/ocean/ocean"
	"encoding/hex"
	"time"

	"github.com/kamva/mgm/v3"
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
	Raw                any        `bson:"raw" json:"raw"`
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

func NewTorrentFromCommitRequest(t *ocean.CommitTorrentRequest) *Torrent {
	now := time.Now()
	r := &Torrent{
		InfoHash:      hex.EncodeToString(t.InfoHash),
		Name:          t.Name,
		Files:         make([]*File, 0, len(t.Files)),
		SearchUpdated: false,
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}
	for _, file := range t.Files {
		r.Files = append(r.Files, NewFileFromProto(file))
		r.Length += file.Length
	}
	return r
}

func NewTorrentFromBTTorrent(t *bittorrent.Torrent) *Torrent {
	now := time.Now()
	ret := &Torrent{
		InfoHash:      hex.EncodeToString(t.InfoHash),
		Name:          t.Name,
		Files:         make([]*File, 0, len(t.Files)),
		SearchUpdated: false,
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}
	for _, file := range t.Files {
		ret.Files = append(ret.Files, NewFileFromBTFile(file))
		ret.Length += file.Length
	}
	return ret
}

func (t *Torrent) Valid() bool {
	if len(t.InfoHash) == 0 {
		return false
	}
	if len(t.Name) == 0 {
		return false
	}
	return true
}

func (t *Torrent) Corrupted() bool {
	for _, f := range t.Files {
		if len(f.Paths) == 0 {
			return true
		}
	}
	return false
}
