package main

import (
	"dht-ocean/bittorrent/tracker"
	"dht-ocean/config"
	"dht-ocean/dao"
	"dht-ocean/storage"
	tracker2 "dht-ocean/tracker"
	"github.com/sirupsen/logrus"
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

	tr, err := tracker.NewUDPTracker(cfg.Tracker)
	if err != nil {
		logrus.Errorf("Failed to create tracker. %v", err)
		return
	}
	err = tr.Start()
	if err != nil {
		logrus.Errorf("Failed to start tracker client. %v", err)
		return
	}
	defer tr.Stop()
	updater := tracker2.NewTrackerUpdater(tr, cfg.TrackerLimit)
	es, err := storage.NewESTorrentStorage(cfg.ES)
	if err != nil {
		logrus.Errorf("Failed to create es torrent storage %v", err)
		return
	}
	updater.AddStorage(es)
	updater.AddStorage(&storage.MongoTorrentStorage{})

	updater.Run()
}
