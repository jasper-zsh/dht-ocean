package main

import (
	"dht-ocean/config"
	"dht-ocean/crawler"
	"dht-ocean/dao"
	"dht-ocean/dht"
	"dht-ocean/model"
	"dht-ocean/storage"
	"github.com/kamva/mgm/v3"
	"github.com/sirupsen/logrus"
	"os"
	"time"
)

func main() {
	cfg, err := config.ReadConfigFromFile("config.yaml")
	if err != nil {
		logrus.Errorf("Failed to read config file. %v", err)
		panic(err)
	}

	nodeID, err := os.ReadFile("node_id")
	if err != nil {
		logrus.Warnf("Cannot read nodeID, generated randomly.")
		nodeID = dht.GenerateNodeID()
		err = os.WriteFile("node_id", nodeID, 0666)
		if err != nil {
			logrus.Errorf("Failed to write nodeID")
		}
	}

	err = dao.InitMongo("dht_ocean", cfg.Mongo)
	if err != nil {
		logrus.Errorf("Failed to init mongodb. %v", err)
		return
	}
	col := mgm.Coll(&model.Torrent{})

	c, err := crawler.NewCrawler(cfg.Listen, nodeID)
	if err != nil {
		logrus.Errorf("Failed to create crawler. %v", err)
		return
	}
	c.SetBootstrapNodes(cfg.BootstrapNodes)
	c.SetInfoHashFilter(func(infoHash []byte) bool {
		t := &model.Torrent{}
		err := col.FindByID(string(infoHash), t)
		if err != nil {
			return true
		}
		return false
	})
	c.AddTorrentHandler(storage.MongoTorrentHandler{})

	err = c.Run()
	defer c.Stop()
	if err != nil {
		logrus.Errorf("Failed to start crawler. %v", err)
		return
	}

	for {
		c.LogStats()
		time.Sleep(time.Second * 5)
	}
}
