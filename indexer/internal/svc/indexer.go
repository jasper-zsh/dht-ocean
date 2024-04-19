package svc

import (
	"context"
	"dht-ocean/common/model"
	"dht-ocean/common/util"
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
	"go.mongodb.org/mongo-driver/mongo/options"
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

	waitTicker  *time.Ticker
	hasTorrent  chan struct{}
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
		waitTicker:  time.NewTicker(emptyWaitTime),
		hasTorrent:  make(chan struct{}, 1),
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
	go i.handleDelayedTorrents()
	err := i.router.Run(i.ctx)
	if err != nil {
		logx.Errorf("Router error: %+v", err)
	}
}

func (i *Indexer) Stop() {
	i.waitTicker.Stop()
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

func (i *Indexer) handleDelayedTorrents() {
	for {
		select {
		case <-i.ctx.Done():
			return
		case <-i.hasTorrent:
			cnt := i.indexDelayedTorrents(i.batchSize)
			if cnt > 0 {
				logx.Infof("Indexed %d delayed torrents", cnt)
				i.waitTicker.Reset(emptyWaitTime)
				util.EmptyChannel(i.hasTorrent)
				i.hasTorrent <- struct{}{}
			}
		case <-i.waitTicker.C:
			util.EmptyChannel(i.hasTorrent)
			i.hasTorrent <- struct{}{}
		}
	}
}

func (i *Indexer) indexDelayedTorrents(limit int64) int {
	if i.delayedCursor == nil {
		return 0
	}
	findMissingStart := time.Now().UnixMilli()
	missingsFromIndex, err := i.missingIndexes(*i.delayedCursor, int(limit))
	if err != nil {
		logx.Errorf("Failed to find missing indexes: %+v", err)
		return 0
	}
	missingsLastUpdatedAt := missingsFromIndex[len(missingsFromIndex)-1].UpdatedAt
	findMissingEnd := time.Now().UnixMilli()
	metricHistogram.Observe(findMissingEnd-findMissingStart, "find_missings_cost")
	if len(missingsFromIndex) == 0 {
		return 0
	}
	ids := make([]string, 0, len(missingsFromIndex))
	for _, t := range missingsFromIndex {
		ids = append(ids, t.InfoHash)
	}
	missingsFromDB := make([]*model.Torrent, 0, limit)
	err = i.torrentCol.SimpleFind(&missingsFromDB, bson.M{
		"_id": bson.M{
			operator.In: ids,
		},
	}, &options.FindOptions{
		Sort: bson.M{
			"updated_at": 1,
		},
	})
	if err != nil {
		logx.Errorf("Failed to fetch missings: %+v", err)
		return 0
	}
	missingFetchAt := time.Now().UnixMilli()
	metricHistogram.Observe(missingFetchAt-findMissingEnd, "fetch_missings_cost")
	missingFromDBSet := make(map[string]struct{})
	for _, t := range missingsFromDB {
		missingFromDBSet[t.InfoHash] = struct{}{}
	}
	// write back to db
	writeBackToDB := make([]*model.Torrent, 0, len(missingsFromIndex))
	for _, t := range missingsFromIndex {
		_, ok := missingFromDBSet[t.InfoHash]
		if !ok && t.Valid() {
			// exists in index but missing in db, write back to db
			writeBackToDB = append(writeBackToDB, t)
		}
	}
	if len(writeBackToDB) > 0 {
		writeBackStart := time.Now().UnixMilli()
		opts := &options.UpdateOptions{}
		opts.SetUpsert(true)
		for _, t := range writeBackToDB {
			err = i.torrentCol.Update(t, opts)
			if err != nil {
				logx.Errorf("Failed to write back to db: %+v", err)
				continue
			}
			metricCounter.Inc("write_back_to_db")
		}
		writeBackAt := time.Now().UnixMilli()
		metricHistogram.Observe(writeBackAt-writeBackStart, "write_back_cost")
	}
	if len(missingsFromDB) > 0 {
		indexMissingStart := time.Now().UnixMilli()
		err = i.batchSaveToIndex(missingsFromDB)
		if err != nil {
			logx.Errorf("Failed to index missings: %+v", err)
			return 0
		}
		indexMissingAt := time.Now().UnixMilli()
		metricHistogram.Observe(indexMissingAt-indexMissingStart, "index_missings_cost")
		if missingsLastUpdatedAt != nil {
			metricGauge.Set(float64(time.Since(*missingsLastUpdatedAt).Seconds()), "delayed_latency_seconds")
		}
		return len(missingsFromDB)
	}
	return 0
}

func (i *Indexer) missingIndexes(cursor time.Time, limit int) ([]*model.Torrent, error) {
	res, err := i.es.Search("torrents").Query(elastic.NewRangeQuery("updated_at").Lt(cursor)).Sort("updated_at", true).Size(limit).Do(i.ctx)
	if err != nil {
		return nil, errors.Trace(err)
	}
	torrents := make([]*model.Torrent, 0, len(res.Hits.Hits))
	for _, hit := range res.Hits.Hits {
		t := model.Torrent{}
		err := json.Unmarshal(hit.Source, &t)
		if err != nil {
			logx.Errorf("Failed to unmarshal torrent from index: %+v", err)
			continue
		}
		torrents = append(torrents, &t)
	}
	return torrents, nil
}

func (i *Indexer) batchSaveToIndex(torrents []*model.Torrent) error {
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
	metricCounter.Add(float64(len(torrents)), "torrent_indexed")
	return nil
}

func (i *Indexer) CountTorrents() (int64, error) {
	cnt, err := i.es.Count("torrents").Do(i.ctx)
	return cnt, err
}
