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
	handler := &CrawlerTorrentHandler{
		trackerLimit: 25,
	}
	c.AddTorrentHandler(handler)
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
	logrus.Infof("Creating torrent %s %s", t.InfoHash, t.Name)
	for _, s := range h.storages {
		s.Store(t)
	}
}

func (h *CrawlerTorrentHandler) RefreshTracker() {
	for {
		h.refreshTracker()
		time.Sleep(time.Second)
	}
}

func (h *CrawlerTorrentHandler) refreshTracker() {
	col := mgm.Coll(&model.Torrent{})
	notTried := make([]*model.Torrent, 0)
	err := col.SimpleFind(&notTried, bson.M{
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
	err = col.SimpleFind(&newRecords, bson.M{
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
	err = col.SimpleFind(&outdated, bson.M{
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

	hashes := make([][]byte, 0, len(records))
	for _, record := range records {
		_, err := col.UpdateByID(nil, record.InfoHash, bson.M{
			operator.Set: bson.M{
				"tracker_last_tried_at": time.Now(),
			},
		})
		if err != nil {
			logrus.Errorf("Failed to update tracker last tried at. %v", err)
			return
		}
		hash, err := hex.DecodeString(record.InfoHash)
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
	now := time.Now()
	for i, r := range scrapes {
		records[i].Seeders = &r.Seeders
		records[i].Leechers = &r.Leechers
		records[i].UpdatedAt = now
		records[i].TrackerUpdatedAt = &now
		logrus.Infof("Updaing torrent %s %s %d:%d", records[i].InfoHash, records[i].Name, r.Seeders, r.Leechers)
		for _, s := range h.storages {
			s.Store(records[i])
		}
	}
}
