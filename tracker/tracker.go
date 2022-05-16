package tracker

import (
	"dht-ocean/bittorrent/tracker"
	"dht-ocean/model"
	"dht-ocean/storage"
	"encoding/hex"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

type TrackerUpdater struct {
	tracker      tracker.Tracker
	trackerLimit int64
	storages     []storage.TorrentStorage
}

func NewTrackerUpdater(tr tracker.Tracker, limit int) *TrackerUpdater {
	r := &TrackerUpdater{
		tracker:      tr,
		trackerLimit: int64(limit),
	}
	return r
}

func (u *TrackerUpdater) AddStorage(storage storage.TorrentStorage) {
	u.storages = append(u.storages, storage)
}

func (u *TrackerUpdater) Run() {
	for {
		u.refreshTracker()
		time.Sleep(time.Second)
	}
}

func (u *TrackerUpdater) refreshTracker() {
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
		Limit: &u.trackerLimit,
	})
	if err != nil {
		logrus.Errorf("Failed to load torrents for tracker. %v", err)
		return
	}
	if len(notTried) >= int(u.trackerLimit) {
		return
	}
	limit := u.trackerLimit - int64(len(notTried))
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
	scrapes, err := u.tracker.Scrape(hashes)
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
		for _, s := range u.storages {
			s.Store(records[i])
		}
	}
}
