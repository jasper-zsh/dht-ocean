package storage

import (
	"dht-ocean/model"
	"github.com/kamva/mgm/v3"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var _ TorrentStorage = (*MongoTorrentStorage)(nil)

type MongoTorrentStorage struct{}

func (m MongoTorrentStorage) Store(t *model.Torrent) {
	col := mgm.Coll(t)
	opts := &options.UpdateOptions{}
	opts.SetUpsert(true)
	err := col.Update(t, opts)
	if err != nil {
		logrus.Errorf("Failed to save torrent %s %s %v", t.InfoHash, t.Name, err)
	}
}
