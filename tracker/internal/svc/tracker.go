package svc

import (
	"context"
	"dht-ocean/common/model"
	"encoding/hex"
	"sync"
	"time"

	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"github.com/sirupsen/logrus"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/metric"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	metricNamespace = "dht_ocean"
	metricSubsystem = "tracker"
)

var (
	metricCounter = metric.NewCounterVec(&metric.CounterVecOpts{
		Namespace: metricNamespace,
		Subsystem: metricSubsystem,
		Name:      "counter",
		Labels:    []string{"type"},
	})
	metricGauge = metric.NewGaugeVec(&metric.GaugeVecOpts{
		Namespace: metricNamespace,
		Subsystem: metricSubsystem,
		Name:      "gauge",
		Labels:    []string{"type"},
	})
)

type TrackerUpdater struct {
	ctx    context.Context
	cancel context.CancelFunc
	svcCtx *ServiceContext

	trackerLimit int64

	torrentCol *mgm.Collection
}

func NewTrackerUpdater(ctx context.Context, svcCtx *ServiceContext, limit int64) *TrackerUpdater {
	r := &TrackerUpdater{
		svcCtx:       svcCtx,
		trackerLimit: limit,
		torrentCol:   mgm.Coll(&model.Torrent{}),
	}
	r.ctx, r.cancel = context.WithCancel(ctx)
	return r
}

func (u *TrackerUpdater) Start() {
	go u.handleResult()
	u.fetch()
}

func (u *TrackerUpdater) Stop() {
	u.cancel()
}

func (u *TrackerUpdater) fetch() {
	for {
		select {
		case <-u.ctx.Done():
			return
		default:
			u.refreshTracker()
		}
	}
}

type trackerUpdate struct {
	Seeders       uint32 `bson:"seeders"`
	Leechers      uint32 `bson:"leechers"`
	SearchUpdated bool   `bson:"search_updated"`
}

func (u *TrackerUpdater) handleResult() {
	for {
		select {
		case <-u.ctx.Done():
			return
		case result := <-u.svcCtx.Tracker.Result():
			// now := time.Now()
			for _, r := range result {
				hash := hex.EncodeToString(r.InfoHash)
				result, err := u.torrentCol.UpdateByID(u.ctx, hash, bson.M{
					operator.Set: trackerUpdate{
						Seeders:       r.Seeders,
						Leechers:      r.Leechers,
						SearchUpdated: false,
					},
					operator.CurrentDate: bson.M{
						"tracker_updated_at": true,
						"updated_at":         true,
					},
				})
				if err != nil {
					logx.Errorf("Failed to update tracker for %s", hash)
					continue
				}
				metricCounter.Add(float64(result.ModifiedCount), "tracker_updated")
			}
		}
	}
}

func (l *TrackerUpdater) getRecords(size int64) ([]*model.Torrent, error) {
	records := make([]*model.Torrent, 0)
	notTried := make([]*model.Torrent, 0)
	err := l.torrentCol.SimpleFind(&notTried, bson.M{
		"tracker_last_tried_at": bson.M{
			operator.Eq: nil,
		},
	}, &options.FindOptions{
		Sort: bson.M{
			"updated_at": 1,
		},
		Limit: &size,
	})
	if err != nil {
		logx.Errorf("Failed to load torrents for tracker. %v", err)
		return nil, err
	}
	records = append(records, notTried...)
	limit := size - int64(len(notTried))
	if limit == 0 {
		return records, nil
	}
	newRecords := make([]*model.Torrent, 0)
	err = l.torrentCol.SimpleFind(&newRecords, bson.M{
		"tracker_updated_at": bson.M{
			operator.Eq: nil,
		},
	}, &options.FindOptions{
		Sort: bson.M{
			"tracker_last_tried_at": 1,
		},
		Limit: &limit,
	})
	if err != nil {
		logrus.Errorf("Failed to load torrents for tracker. %v", err)
		return nil, err
	}
	records = append(records, newRecords...)
	limit = limit - int64(len(newRecords))
	if limit == 0 {
		return records, nil
	}
	outdated := make([]*model.Torrent, 0)
	age := time.Now().Add(-6 * time.Hour)
	err = l.torrentCol.SimpleFind(&outdated, bson.M{
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
		return nil, err
	}
	if len(outdated) > 0 {
		trackerLastUpdated := outdated[len(outdated)-1].TrackerUpdatedAt
		if trackerLastUpdated != nil {
			metricGauge.Set(float64(time.Since(*trackerLastUpdated).Seconds()), "tracker_latency_seconds")
		}
	}
	records = append(records, outdated...)
	return records, nil
}

func (u *TrackerUpdater) refreshTracker() {
	records, err := u.getRecords(u.trackerLimit)
	if err != nil {
		logx.Errorf("Failed to fetch torrents for tracker: %+v", err)
		return
	}
	now := time.Now()
	wait := sync.WaitGroup{}
	for _, r := range records {
		wait.Add(1)
		infoHash := r.InfoHash
		go func() {
			defer wait.Done()
			_, err := u.torrentCol.UpdateByID(u.ctx, infoHash, bson.M{
				operator.Set: bson.M{
					"tracker_last_tried_at": now,
				},
			})
			if err != nil {
				logx.Errorf("Failed to update tracker last tries time for %s", r.InfoHash)
			}
		}()
	}
	wait.Wait()
	infoHashes := make([][]byte, 0, len(records))
	for _, record := range records {
		hash, err := hex.DecodeString(record.InfoHash)
		if err != nil {
			logrus.Errorf("broken torrent record, skip tracker scrape")
			continue
		}
		infoHashes = append(infoHashes, hash)
	}
	err = u.svcCtx.Tracker.Scrape(infoHashes)
	if err != nil {
		logrus.Warnf("Failed to scrape %d torrents from tracker. %v", len(infoHashes), err)
		return
	}
}
