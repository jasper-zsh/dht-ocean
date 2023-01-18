package bittorrent

type Torrent struct {
	InfoHash    []byte         `json:"info_hash"`
	Name        string         `mapstructure:"name" json:"name"`
	PieceLength int64          `mapstructure:"piece length" json:"-"`
	Pieces      []byte         `mapstructure:"pieces" json:"-"`
	Publisher   string         `mapstructure:"publisher" json:"publisher,omitempty"`
	Files       []*File        `mapstructure:"files" json:"files,omitempty"`
	Source      string         `mapstructure:"source" json:"source,omitempty"`
	Other       map[string]any `mapstructure:",remain" json:"other,omitempty"`
}

type TorrentHandler interface {
	HandleTorrent(torrent *Torrent)
}
