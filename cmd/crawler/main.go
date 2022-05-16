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
	"github.com/kamva/mgm/v3/operator"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
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
		err := col.FindByID(hex.EncodeToString(infoHash), t)
		if err != nil {
			return true
		}
		return false
	})
	handler := &CrawlerTorrentHandler{}
	c.AddTorrentHandler(handler)
	tr, err := tracker.NewUDPTracker(cfg.Tracker)
	if err != nil {
		logrus.Errorf("Failed to create tracker. %v", err)
		return
	}
	handler.SetTracker(tr)
	es, err := storage.NewESTorrentStorage(cfg.ES)
	if err != nil {
		logrus.Errorf("Failed to create es torrent storage %v", err)
		return
	}
	handler.AddStorage(es)
	handler.AddStorage(&storage.MongoTorrentStorage{})

	err = c.Run()
	defer c.Stop()
	if err != nil {
		logrus.Errorf("Failed to start crawler. %v", err)
		return
	}
	go handler.RefreshTracker()

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

func (h *CrawlerTorrentHandler) SetTracker(tr tracker.Tracker) {
	h.tracker = tr
}

func (h *CrawlerTorrentHandler) AddStorage(s storage.TorrentStorage) {
	h.storages = append(h.storages, s)
}

func (h *CrawlerTorrentHandler) HandleTorrent(torrent *bittorrent.Torrent) {
	t := model.NewTorrentFromCrawler(torrent)
	for _, s := range h.storages {
		s.Store(t)
	}
}

func (h *CrawlerTorrentHandler) RefreshTracker() {
	for {
		h.refreshTracker()
	}
}

func (h *CrawlerTorrentHandler) refreshTracker() {
	notTried := make([]*model.Torrent, 0)
	err := mgm.Coll(&model.Torrent{}).SimpleFind(&notTried, bson.M{
		"tracker_last_tried_at": bson.M{
			operator.Eq: bson.TypeNull,
		},
	}, &options.FindOptions{
		Sort: bson.M{
			"updated_at": 1,
		},
		Limit: &h.trackerLimit,
	})
	if err != nil {
		logrus.Errorf("Failed to load torrents for tracker. %v", err)
		return
	}
	if len(notTried) >= int(h.trackerLimit) {
		return
	}
	limit := h.trackerLimit - int64(len(notTried))
	newRecords := make([]*model.Torrent, 0)
	err = mgm.Coll(&model.Torrent{}).SimpleFind(&newRecords, bson.M{
		"tracker_updated_at": bson.M{
			operator.Eq: bson.TypeNull,
		},
	}, &options.FindOptions{
		Sort: bson.M{
			"tracker_last_tried_at": 1,
		},
		Limit: &limit,
	})
	if err != nil {
		logrus.Errorf("Failed to load torrents for tracker. %v", err)
		return
	}
	if len(newRecords) >= int(limit) {
		return
	}
	limit = limit - int64(len(newRecords))
	outdated := make([]*model.Torrent, 0)
	age := time.Now().Add(-6 * time.Hour)
	err = mgm.Coll(&model.Torrent{}).SimpleFind(&outdated, bson.M{
		"tracker_updated_at": bson.M{
			operator.Lte: age,
		},
	}, &options.FindOptions{
		Sort: bson.M{
			"tracker_updated_at": 1,
		},
		Limit: &limit,
	})
	if err != nil {
		logrus.Errorf("Failed to load torrents for tracker. %v", err)
		return
	}
	records := append(notTried, newRecords...)
	records = append(records, outdated...)
	for i := 0; i*h.trackerBatchSize < len(records); i++ {
		var end int
		if len(records) < (i+1)*h.trackerBatchSize {
			end = len(records)
		} else {
			end = (i + 1) * h.trackerBatchSize
		}
		chunk := records[i*h.trackerBatchSize : end]
		hashes := make([][]byte, len(chunk))
		for _, chunk := range chunk {
			hash, err := hex.DecodeString(chunk.InfoHash)
			if err != nil {
				logrus.Errorf("broken torrent record, skip tracker scrape")
				return
			}
			hashes = append(hashes, hash)
		}
		scrapes, err := h.tracker.Scrape(hashes)
		if err != nil {
			logrus.Warnf("Failed to scrape %d torrents from tracker. %v", len(hashes), err)
			return
		}
		for i, r := range scrapes {
			records[i].Seeders = &r.Seeders
			records[i].Leechers = &r.Leechers
			for _, s := range h.storages {
				s.Store(records[i])
			}
		}
	}
}
