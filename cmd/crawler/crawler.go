package main

import (
	"dht-ocean/bittorrent"
	"dht-ocean/bittorrent/tracker"
	"dht-ocean/config"
	"dht-ocean/crawler"
	"dht-ocean/dao"
	"dht-ocean/dht"
	"dht-ocean/model"
	"dht-ocean/storage"
	"encoding/hex"
	"github.com/kamva/mgm/v3"
	"github.com/sirupsen/logrus"
	"os"
	"time"
)

func main() {
	//logrus.SetLevel(logrus.DebugLevel)

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
	c.SetMaxQueueSize(cfg.MaxQueueSize)
	c.SetBootstrapNodes(cfg.BootstrapNodes)
	c.SetInfoHashFilter(func(infoHash []byte) bool {
		t := &model.Torrent{}
		err := col.FindByID(hex.EncodeToString(infoHash), t)
		if err != nil {
			return true
		}
		return false
	})
	handler := &CrawlerTorrentHandler{
		trackerLimit: 25,
	}
	c.AddTorrentHandler(handler)

	handler.AddStorage(&storage.MongoTorrentStorage{})

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

var _ bittorrent.TorrentHandler = (*CrawlerTorrentHandler)(nil)

type CrawlerTorrentHandler struct {
	tracker          tracker.Tracker
	storages         []storage.TorrentStorage
	trackerLimit     int64
	trackerBatchSize int
}

func (h *CrawlerTorrentHandler) AddStorage(s storage.TorrentStorage) {
	h.storages = append(h.storages, s)
}

func (h *CrawlerTorrentHandler) HandleTorrent(torrent *bittorrent.Torrent) {
	t := model.NewTorrentFromCrawler(torrent)
	logrus.Infof("Creating torrent %s %s", t.InfoHash, t.Name)
	for _, s := range h.storages {
		s.Store(t)
	}
}
