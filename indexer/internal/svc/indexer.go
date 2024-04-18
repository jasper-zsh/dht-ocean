package svc

import (
	"context"
	"dht-ocean/common/model"
	"dht-ocean/common/util"
	"dht-ocean/indexer/internal/config"
	"encoding/json"
	"time"

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
	emptyWaitTime   = 5 * time.Second
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

	waitTicker  *time.Ticker
	statsTicker *time.Ticker
	hasTorrent  chan struct{}
}

func NewIndexer(ctx context.Context, conf *config.Config) (*Indexer, error) {
	indexer := &Indexer{
		waitTicker:  time.NewTicker(emptyWaitTime),
		statsTicker: time.NewTicker(1 * time.Second),
		hasTorrent:  make(chan struct{}, 1),
		torrentCol:  mgm.Coll(&model.Torrent{}),
		batchSize:   conf.BatchSize,
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
		case <-i.hasTorrent:
			cnt, lastUpdatedAt := i.indexTorrents(int64(i.batchSize))
			if cnt > 0 {
				logx.Infof("Indexed %d torrents", cnt)
				i.waitTicker.Reset(emptyWaitTime)
				util.EmptyChannel(i.hasTorrent)
				i.hasTorrent <- struct{}{}
			} else {
				logx.Infof("Index done, wait for next tick")
			}
			if lastUpdatedAt != nil {
				metricGauge.Set(float64(time.Since(*lastUpdatedAt).Seconds()), "index_latency_seconds")
			}
		case <-i.waitTicker.C:
			util.EmptyChannel(i.hasTorrent)
			i.hasTorrent <- struct{}{}
		}
	}
}

func (i *Indexer) Stop() {
	i.waitTicker.Stop()
	i.cancel()
}

func (i *Indexer) indexTorrents(limit int64) (int, *time.Time) {
	startAt := time.Now().UnixMilli()
	col := mgm.Coll(&model.Torrent{})
	records := make([]*model.Torrent, 0, limit)
	err := col.SimpleFind(&records, bson.M{
		"search_updated": false,
	}, &options.FindOptions{
		Sort: bson.M{
			"updated_at": 1,
		},
		Limit: &limit,
	})
	if err != nil {
		logx.Errorf("Failed to find records to index. %+v", err)
		return 0, nil
	}
	fetchAt := time.Now().UnixMilli()
	metricHistogram.Observe(fetchAt-startAt, "fetch_cost")
	if len(records) == 0 {
		return 0, nil
	}
	lastUpdatedAt := records[len(records)-1].UpdatedAt
	err = i.batchSaveToIndex(records)
	if err != nil {
		return 0, nil
	}
	indexAt := time.Now().UnixMilli()
	metricHistogram.Observe(indexAt-fetchAt, "index_cost")
	if lastUpdatedAt != nil {
		missingsFromIndex, err := i.missingIndexes(*lastUpdatedAt, int(limit))
		if err != nil {
			logx.Errorf("Failed to find missing indexes: %+v", err)
			return len(records), lastUpdatedAt
		}
		missingsLastUpdatedAt := missingsFromIndex[len(missingsFromIndex)-1].UpdatedAt
		if lastUpdatedAt != nil && missingsLastUpdatedAt != nil && missingsLastUpdatedAt.Before(*lastUpdatedAt) {
			lastUpdatedAt = missingsLastUpdatedAt
		}
		missingAt := time.Now().UnixMilli()
		metricHistogram.Observe(missingAt-indexAt, "find_missings_cost")
		if len(missingsFromIndex) == 0 {
			return len(records), lastUpdatedAt
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
			return len(records), lastUpdatedAt
		}
		missingFetchAt := time.Now().UnixMilli()
		metricHistogram.Observe(missingFetchAt-missingAt, "fetch_missings_cost")
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
				return len(records), lastUpdatedAt
			}
			indexMissingAt := time.Now().UnixMilli()
			metricHistogram.Observe(indexMissingAt-indexMissingStart, "index_missings_cost")
			return len(records) + len(missingsFromDB), lastUpdatedAt
		}
	}
	return len(records), lastUpdatedAt
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
