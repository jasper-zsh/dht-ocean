package svc

import (
	"context"
	"dht-ocean/common/model"
	"dht-ocean/indexer/internal/config"
	"encoding/json"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-amqp/v2/pkg/amqp"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/juju/errors"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"github.com/olivere/elastic/v7"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/metric"
	"go.mongodb.org/mongo-driver/bson"
)

const (
	emptyWaitTime   = 10 * time.Second
	metricNamespace = "dht_ocean"
	metricSubsystem = "indexer"
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
	metricHistogram = metric.NewHistogramVec(&metric.HistogramVecOpts{
		Namespace: metricNamespace,
		Subsystem: metricSubsystem,
		Name:      "histogram",
		Labels:    []string{"type"},
	})
)

type Indexer struct {
	ctx    context.Context
	cancel context.CancelFunc

	torrentCol *mgm.Collection
	es         *elastic.Client

	batchSize     int64
	batch         []*model.Torrent
	delayedCursor *time.Time

	publisher message.Publisher
	router    *message.Router

	statsTicker *time.Ticker
}

const (
	handlerNameIndexer = "indexer"
)

func NewIndexer(ctx context.Context, conf *config.Config) (*Indexer, error) {
	indexer := &Indexer{
		statsTicker: time.NewTicker(1 * time.Second),
		torrentCol:  mgm.Coll(&model.Torrent{}),
		batchSize:   conf.BatchSize,
		batch:       make([]*model.Torrent, 0, conf.BatchSize),
	}
	indexer.ctx, indexer.cancel = context.WithCancel(ctx)
	var err error
	indexer.es, err = elastic.NewClient(
		elastic.SetURL(conf.ElasticSearch),
		elastic.SetSniff(false),
	)
	if err != nil {
		return nil, errors.Trace(err)
	}

	logger := watermill.NewStdLogger(false, false)
	amqpConfig := amqp.NewDurablePubSubConfig(conf.AMQP, amqp.GenerateQueueNameConstant("indexer"))
	amqpConfig.Consume.Qos.PrefetchCount = conf.AMQPPreFetch
	indexer.publisher, err = amqp.NewPublisher(amqpConfig, logger)
	if err != nil {
		return nil, errors.Trace(err)
	}
	subscriber, err := amqp.NewSubscriber(amqpConfig, logger)
	if err != nil {
		return nil, errors.Trace(err)
	}
	err = subscriber.SubscribeInitialize(model.TopicTrackerUpdated)
	if err != nil {
		return nil, errors.Trace(err)
	}
	indexer.router, err = message.NewRouter(message.RouterConfig{}, logger)
	if err != nil {
		return nil, errors.Trace(err)
	}
	indexer.router.AddNoPublisherHandler(handlerNameIndexer, model.TopicNewTorrent, subscriber, indexer.consumeTorrent)

	return indexer, nil
}

func (i *Indexer) stats() {
	for {
		select {
		case <-i.ctx.Done():
			return
		case <-i.statsTicker.C:
			count, err := i.CountTorrents()
			if err != nil {
				logx.Errorf("Failed to count torrents in index: %+v", err)
			} else {
				metricGauge.Set(float64(count), "torrent_indexed")
			}
		}
	}
}

func (i *Indexer) Start() {
	go i.stats()
	err := i.router.Run(i.ctx)
	if err != nil {
		logx.Errorf("Router error: %+v", err)
	}
}

func (i *Indexer) Stop() {
	i.cancel()
}

func (i *Indexer) consumeTorrent(msg *message.Message) error {
	t := &model.Torrent{}
	err := json.Unmarshal(msg.Payload, t)
	if err != nil {
		return errors.Trace(err)
	}
	if len(i.batch) < int(i.batchSize)-1 {
		i.batch = append(i.batch, t)
	} else {
		// keep idempotent
		err := i.batchSaveToIndex(append(i.batch, t))
		if err != nil {
			return errors.Trace(err)
		}
		i.delayedCursor = i.batch[0].UpdatedAt
		i.batch = make([]*model.Torrent, 0, i.batchSize)
	}
	return nil
}

func (i *Indexer) batchSaveToIndex(torrents []*model.Torrent) error {
	startAt := time.Now().UnixMilli()
	reqs := make([]elastic.BulkableRequest, 0, len(torrents))
	for _, torrent := range torrents {
		torrent.SearchUpdated = true
		reqs = append(reqs, elastic.NewBulkUpdateRequest().Index("torrents").Id(torrent.InfoHash).Doc(torrent).DocAsUpsert(true))
	}
	_, err := i.es.Bulk().Add(reqs...).Do(i.ctx)
	if err != nil {
		logx.Errorf("Failed to index %d torrents. %+v", len(torrents), err)
		return err
	}
	col := mgm.Coll(&model.Torrent{})
	for _, torrent := range torrents {
		_, err = col.UpdateByID(context.TODO(), torrent.InfoHash, bson.M{
			operator.Set: bson.M{
				"search_updated": true,
			},
		})
		if err != nil {
			logx.Errorf("Failed to update search_updated")
			return err
		}
	}
	endAt := time.Now().UnixMilli()
	metricHistogram.Observe(endAt-startAt, "index_cost")
	metricCounter.Add(float64(len(torrents)), "torrent_indexed")
	return nil
}

func (i *Indexer) CountTorrents() (int64, error) {
	cnt, err := i.es.Count("torrents").Do(i.ctx)
	return cnt, err
}
