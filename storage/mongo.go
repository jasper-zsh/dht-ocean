package storage

import (
	"dht-ocean/bittorrent"
	"dht-ocean/model"
	"github.com/kamva/mgm/v3"
	"github.com/sirupsen/logrus"
)

var _ bittorrent.TorrentHandler = (*MongoTorrentHandler)(nil)

type MongoTorrentHandler struct{}

func (m MongoTorrentHandler) HandleTorrent(torrent *bittorrent.Torrent) {
	t := model.NewTorrentFromCrawler(torrent)
	col := mgm.Coll(t)
	err := col.Create(t)
	if err != nil {
		logrus.Errorf("Failed to save torrent %s %s %v", t.InfoHash, t.Name, err)
	} else {
		logrus.Infof("Saved torrent %s %s", t.InfoHash, t.Name)
	}
}
