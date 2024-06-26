package svc

import (
	"context"
	"dht-ocean/common/model"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-amqp/v2/pkg/amqp"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/juju/errors"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"github.com/sirupsen/logrus"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/metric"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
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
	batch        []*model.Torrent

	torrentCol *mgm.Collection

	publisher message.Publisher
	router    *message.Router

	needUpdate chan struct{}
}

const (
	handlerNameTrackerPrefix = "tracker_"
)

func NewTrackerUpdater(ctx context.Context, svcCtx *ServiceContext, limit int64) (*TrackerUpdater, error) {
	r := &TrackerUpdater{
		svcCtx:       svcCtx,
		trackerLimit: limit,
		batch:        make([]*model.Torrent, 0, limit),
		torrentCol:   mgm.Coll(&model.Torrent{}),
		needUpdate:   make(chan struct{}, 1),
	}
	r.ctx, r.cancel = context.WithCancel(ctx)

	amqpConfig := amqp.NewDurablePubSubConfig(svcCtx.Config.AMQP, amqp.GenerateQueueNameTopicNameWithSuffix("tracker"))
	amqpConfig.Consume.Qos.PrefetchCount = svcCtx.Config.AMQPPreFetch
	var err error
	logger := watermill.NewStdLogger(false, false)
	r.publisher, err = amqp.NewPublisher(amqpConfig, logger)
	if err != nil {
		return nil, errors.Trace(err)
	}
	subscriber, err := amqp.NewSubscriber(amqpConfig, logger)
	if err != nil {
		return nil, errors.Trace(err)
	}
	r.router, err = message.NewRouter(message.RouterConfig{}, logger)
	if err != nil {
		return nil, errors.Trace(err)
	}
	r.router.AddNoPublisherHandler(handlerNameTrackerPrefix+"new", model.TopicNewTorrent, subscriber, r.handleNewTorrent)

	return r, nil
}

func (u *TrackerUpdater) Start() {
	go u.handleResult()
	go u.fetch()
	u.needUpdate <- struct{}{}
	err := u.router.Run(u.ctx)
	if err != nil {
		logx.Errorf("Router error: %+v", err)
	}
}

func (u *TrackerUpdater) Stop() {
	u.cancel()
}

func (u *TrackerUpdater) fetch() {
	for {
		select {
		case <-u.ctx.Done():
			return
		case <-u.needUpdate:
			u.refreshTracker()
		}
	}
}

func (u *TrackerUpdater) handleNewTorrent(msg *message.Message) error {
	t := &model.Torrent{}
	err := json.Unmarshal(msg.Payload, t)
	if err != nil {
		return errors.Trace(err)
	}
	if len(u.batch) < int(u.trackerLimit)-1 {
		u.batch = append(u.batch, t)
	} else {
		b := append(u.batch, t)
		infoHashes := make([][]byte, 0, len(b))
		for _, torrent := range b {
			hash, err := hex.DecodeString(torrent.InfoHash)
			if err != nil {
				return errors.Trace(err)
			}
			infoHashes = append(infoHashes, hash)
		}
		err := u.svcCtx.Tracker.Scrape(infoHashes)
		if err != nil {
			return errors.Trace(err)
		}
		u.batch = make([]*model.Torrent, 0, u.trackerLimit)
	}

	return nil
}

type trackerUpdate struct {
	Seeders          uint32    `bson:"seeders"`
	Leechers         uint32    `bson:"leechers"`
	SearchUpdated    bool      `bson:"search_updated"`
	TrackerUpdatedAt time.Time `bson:"tracker_updated_at"`
	UpdatedAt        time.Time `bson:"updated_at"`
}

func (u *TrackerUpdater) handleResult() {
	for {
		select {
		case <-u.ctx.Done():
			return
		case result := <-u.svcCtx.Tracker.Result():
			now := time.Now()
			for _, r := range result {
				hash := hex.EncodeToString(r.InfoHash)
				result := u.torrentCol.FindOneAndUpdate(u.ctx, bson.M{
					"_id": hash,
				}, bson.M{
					operator.Set: &trackerUpdate{
						Seeders:          r.Seeders,
						Leechers:         r.Leechers,
						SearchUpdated:    false,
						TrackerUpdatedAt: now,
						UpdatedAt:        now,
					},
				}, options.FindOneAndUpdate().SetReturnDocument(options.After))
				err := result.Err()
				if err != nil {
					if err == mongo.ErrNoDocuments {
						logx.Infof("Torrent %s not found", hash)
						continue
					} else {
						logx.Errorf("Failed to update %s tracker: %+v", hash, err)
						continue
					}
				}
				torrent := &model.Torrent{}
				err = result.Decode(torrent)
				if err != nil {
					logx.Errorf("Failed to unmarshal torrent: %+v", err)
					continue
				}
				raw, err := json.Marshal(torrent)
				if err != nil {
					logx.Errorf("Failed to marshal torrent to json: %+v", err)
					continue
				}
				msg := message.NewMessage(watermill.NewUUID(), raw)
				err = u.publisher.Publish(model.TopicTrackerUpdated, msg)
				if err != nil {
					logx.Errorf("Failed to publish tracker updated: %+v", err)
					continue
				}
				metricCounter.Inc("tracker_updated")
			}
		}
	}
}

func (l *TrackerUpdater) getRecords(size int64) ([]*model.Torrent, error) {
	records := make([]*model.Torrent, 0)
	outdated := make([]*model.Torrent, 0)
	limit := size
	age := time.Now().Add(-6 * time.Hour)
	err := l.torrentCol.SimpleFind(&outdated, bson.M{
		"tracker_last_tried_at": bson.M{
			operator.Lte: age,
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
	if len(outdated) > 0 {
		trackerLastUpdated := outdated[len(outdated)-1].TrackerLastTriedAt
		if trackerLastUpdated != nil {
			metricGauge.Set(float64(time.Since(*trackerLastUpdated).Seconds()), "tracker_latency_seconds")
		}
	}
	records = append(records, outdated...)
	return records, nil
}

func (u *TrackerUpdater) refreshTracker() {
	var records []*model.Torrent
	var err error
	defer func() {
		if len(records) > 0 {
			u.needUpdate <- struct{}{}
		} else {
			logx.Infof("No torrent need to update tracker, wait for 10 seconds...")
			time.AfterFunc(10*time.Second, func() {
				u.needUpdate <- struct{}{}
			})
		}
	}()
	records, err = u.getRecords(u.trackerLimit)
	if err != nil {
		logx.Errorf("Failed to fetch torrents for tracker: %+v", err)
		return
	}

	ids := make([]string, 0, len(records))
	infoHashes := make([][]byte, 0, len(records))
	for _, r := range records {
		ids = append(ids, r.InfoHash)
		b, _ := hex.DecodeString(r.InfoHash)
		infoHashes = append(infoHashes, b)
	}
	_, err = u.torrentCol.UpdateMany(u.ctx, bson.M{
		"_id": bson.M{
			operator.In: ids,
		},
	}, bson.M{
		operator.CurrentDate: bson.M{
			"tracker_last_tried_at": true,
		},
	})
	if err != nil {
		logx.Errorf("Failed to update tracker last tried at: %+v", err)
		return
	}
	err = u.svcCtx.Tracker.Scrape(infoHashes)
	if err != nil {
		logx.Errorf("Failed to scrape update tracker: %+v", err)
	}
}

func (u *TrackerUpdater) Recover() error {
	for {
		torrents := make([]*model.Torrent, 0)
		err := u.torrentCol.SimpleFind(&torrents, bson.M{
			"tracker_last_tried_at": nil,
		}, &options.FindOptions{
			Limit: &u.trackerLimit,
		})
		if err != nil {
			return errors.Trace(err)
		}
		if len(torrents) == 0 {
			return nil
		}
		ids := make([]string, 0, len(torrents))
		for _, torrent := range torrents {
			raw, err := json.Marshal(torrent)
			if err != nil {
				return errors.Trace(err)
			}
			ids = append(ids, torrent.InfoHash)
			msg := message.NewMessage(watermill.NewUUID(), raw)
			err = u.publisher.Publish(model.TopicNewTorrent, msg)
			if err != nil {
				return errors.Trace(err)
			}
		}
		_, err = u.torrentCol.UpdateMany(u.ctx, bson.M{
			"_id": bson.M{
				operator.In: ids,
			},
		}, bson.M{
			operator.CurrentDate: bson.M{
				"tracker_last_tried_at": true,
			},
		})
		if err != nil {
			logx.Errorf("Failed to update tracker last tried at: %+v", err)
			return errors.Trace(err)
		}
		logx.Infof("Recovered %d torrents", len(torrents))
	}
}
