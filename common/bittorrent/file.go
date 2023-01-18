package bittorrent

type File struct {
	ED2K     []byte         `mapstructure:"ed2k" json:"ed2k,omitempty"`
	FileHash []byte         `mapstructure:"filehash" json:"file_hash,omitempty"`
	Length   int64          `mapstructure:"length" json:"length,omitempty"`
	Path     []string       `mapstructure:"path" json:"path,omitempty"`
	Other    map[string]any `mapstructure:",remain" json:"other,omitempty"`
}
