package main

import (
	"dht-ocean/config"
	"dht-ocean/dao"
	"dht-ocean/model"
	"dht-ocean/storage"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

func main() {
	cfg, err := config.ReadConfigFromFile("config.yaml")
	if err != nil {
		logrus.Errorf("Failed to read config file. %v", err)
		panic(err)
	}

	err = dao.InitMongo("dht_ocean", cfg.Mongo)
	if err != nil {
		logrus.Errorf("Failed to init mongodb. %v", err)
		return
	}

	es, err := storage.NewESTorrentStorage(cfg.ES)
	if err != nil {
		logrus.Errorf("Failed to create es torrent storage %v", err)
		return
	}

	for {
		cnt := indexTorrents(es, 100)
		if cnt == 0 {
			time.Sleep(time.Second * 5)
		}
	}
}

func indexTorrents(s storage.TorrentStorage, limit int64) int {
	col := mgm.Coll(&model.Torrent{})
	records := make([]*model.Torrent, limit)
	err := col.SimpleFind(&records, bson.M{
		"search_updated": false,
	}, &options.FindOptions{
		Sort: bson.M{
			"updated_at": 1,
		},
		Limit: &limit,
	})
	if err != nil {
		logrus.Errorf("Failed to find records to index. %+v", err)
		return 0
	}
	for _, record := range records {
		record.SearchUpdated = true
		err := s.Store(record)
		if err != nil {
			return 0
		}
		_, err = col.UpdateByID(nil, record.InfoHash, bson.M{
			operator.Set: bson.M{
				"search_updated": true,
			},
		})
		if err != nil {
			logrus.Errorf("Failed to update search_updated")
			return 0
		}
	}
	logrus.Infof("Indexed %d torrents.", len(records))
	return len(records)
}
