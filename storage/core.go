package storage

import "dht-ocean/model"

type TorrentStorage interface {
	Store(torrent *model.Torrent)
}
