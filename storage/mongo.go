package storage

import (
	"dht-ocean/model"
	"github.com/kamva/mgm/v3"
	"github.com/sirupsen/logrus"
)

var _ TorrentStorage = (*MongoTorrentStorage)(nil)

type MongoTorrentStorage struct{}

func (m MongoTorrentStorage) Store(t *model.Torrent) {
	col := mgm.Coll(t)
	err := col.Create(t)
	if err != nil {
		logrus.Errorf("Failed to save torrent %s %s %v", t.InfoHash, t.Name, err)
	} else {
		logrus.Infof("Saved torrent %s %s", t.InfoHash, t.Name)
	}
}
