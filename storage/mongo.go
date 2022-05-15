package storage

import (
	"dht-ocean/bittorrent"
	"dht-ocean/model"
	"github.com/kamva/mgm/v3"
)

var _ bittorrent.TorrentHandler = (*MongoTorrentHandler)(nil)

type MongoTorrentHandler struct{}

func (m MongoTorrentHandler) HandleTorrent(torrent *bittorrent.Torrent) {
	t := model.NewTorrentFromCrawler(torrent)
	_ = mgm.Coll(t)
}
