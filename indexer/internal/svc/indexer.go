package svc

import (
	"context"
	"dht-ocean/common/amqp"
	"dht-ocean/common/model"
	"dht-ocean/indexer/internal/config"
	"encoding/json"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/juju/errors"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"github.com/olivere/elastic/v7"
	"github.com/rabbitmq/amqp091-go"
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

	batchSize int64
	batch     []*model.Torrent

	delayedCursor *time.Time

	publisher  message.Publisher
	amqpClient *amqp.Client
	deliveries <-chan amqp091.Delivery

	statsTicker *time.Ticker
}

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

	client, err := amqp.NewClient(conf.AMQP)
	if err != nil {
		return nil, errors.Trace(err)
	}
	err = client.SetQos(int(conf.BatchSize))
	if err != nil {
		return nil, errors.Trace(err)
	}
	indexer.amqpClient = client
	queueName := "indexer"
	err = client.DeclareQueue(queueName)
	if err != nil {
		return nil, errors.Trace(err)
	}
	err = client.QueueBind(queueName, model.TopicNewTorrent)
	if err != nil {
		return nil, errors.Trace(err)
	}
	err = client.QueueBind(queueName, model.TopicTrackerUpdated)
	if err != nil {
		return nil, errors.Trace(err)
	}
	indexer.deliveries, err = client.Consume(ctx, queueName, false)
	if err != nil {
		return nil, errors.Trace(err)
	}

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
	for {
		select {
		case <-i.ctx.Done():
			return
		case delivery := <-i.deliveries:
			t := &model.Torrent{}
			err := json.Unmarshal(delivery.Body, t)
			if err != nil {
				logx.Infof("Failed to unmarshal torrent, drop. %s %+v", delivery.Body, err)
				err := delivery.Ack(false)
				if err != nil {
					panic(err)
				}
				continue
			}
			i.batch = append(i.batch, t)
			if len(i.batch) >= int(i.batchSize) {
				err := i.batchSaveToIndex(i.batch)
				if err != nil {
					logx.Errorf("Failed to index %d torrents, nacked: %+v", len(i.batch), err)
					err := delivery.Nack(true, true)
					if err != nil {
						panic(err)
					}
				} else {
					i.delayedCursor = i.batch[0].UpdatedAt
					err := delivery.Ack(true)
					if err != nil {
						panic(err)
					}
				}
				i.batch = make([]*model.Torrent, 0, i.batchSize)
			}
		}
	}
}

func (i *Indexer) Stop() {
	i.cancel()
}

func (i *Indexer) batchSaveToIndex(torrents []*model.Torrent) error {
	startAt := time.Now().UnixMilli()
	reqs := make([]elastic.BulkableRequest, 0, len(torrents))
	ids := make([]string, 0, len(torrents))
	for _, torrent := range torrents {
		torrent.SearchUpdated = true
		ids = append(ids, torrent.InfoHash)
		reqs = append(reqs, elastic.NewBulkUpdateRequest().Index("torrents").Id(torrent.InfoHash).Doc(torrent).DocAsUpsert(true))
	}
	_, err := i.es.Bulk().Add(reqs...).Do(i.ctx)
	if err != nil {
		logx.Errorf("Failed to index %d torrents. %+v", len(torrents), err)
		return err
	}
	indexEndAt := time.Now().UnixMilli()
	metricHistogram.Observe(indexEndAt-startAt, "index_cost")
	col := mgm.Coll(&model.Torrent{})
	result, err := col.UpdateMany(i.ctx, bson.M{
		"_id": bson.M{
			operator.In: ids,
		},
	}, bson.M{
		operator.Set: bson.M{
			"search_updated": true,
		},
	})
	if err != nil {
		logx.Errorf("Failed to update search_updated")
		return errors.Trace(err)
	}
	if result.MatchedCount != int64(len(torrents)) {
		logx.Infof("Updated less search flag than indexed: %d %d", result.MatchedCount, len(torrents))
	}
	searchUpdatedAt := time.Now().UnixMilli()
	metricHistogram.Observe(searchUpdatedAt-indexEndAt, "indexed_flag_cost")
	metricCounter.Add(float64(len(torrents)), "torrent_indexed")
	return nil
}

func (i *Indexer) CountTorrents() (int64, error) {
	cnt, err := i.es.Count("torrents").Do(i.ctx)
	return cnt, err
}
