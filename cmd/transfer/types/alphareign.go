package types

import (
	"dht-ocean/model"
	"encoding/json"
	"strings"
	"time"
)

type AlphaReign struct {
	InfoHash       string `gorm:"column:infohash"`
	Name           string
	Files          string
	Tags           string
	Type           string
	Length         int64
	Created        time.Time
	Updated        time.Time
	Seeders        *int
	Leechers       *int
	TrackerUpdated *time.Time `gorm:"column:trackerUpdated"`
	SearchUpdate   bool       `gorm:"column:searchUpdate"`
	SearchUpdated  *time.Time `gorm:"searchUpdated"`
	TrackerLastTry *time.Time `gorm:"column:trackerLastTry"`
}

func (a *AlphaReign) ToTorrent() *model.Torrent {
	n := &model.Torrent{
		InfoHash:           a.InfoHash,
		Name:               a.Name,
		Type:               a.Type,
		Length:             a.Length,
		CreatedAt:          a.Created,
		UpdatedAt:          a.Updated,
		Seeders:            a.Seeders,
		Leechers:           a.Leechers,
		TrackerUpdatedAt:   a.TrackerUpdated,
		SearchUpdated:      a.SearchUpdate,
		TrackerLastTriedAt: a.TrackerLastTry,
	}
	if len(a.Tags) > 0 {
		tags := strings.Split(a.Tags, ",")
		n.Tags = tags
	}
	files := make([]AlphaReignFile, 0)
	err := json.Unmarshal([]byte(a.Files), &files)
	if err == nil {
		for _, f := range files {
			n.Files = append(n.Files, f.ToFile())
		}
	}
	return n
}

type AlphaReignFile struct {
	Length int64    `json:"length"`
	Path   []string `json:"path"`
}

func (f *AlphaReignFile) ToFile() *model.File {
	r := &model.File{}
	r.Length = f.Length
	if len(f.Path) > 0 {
		r.Paths = f.Path
	}
	return r
}
