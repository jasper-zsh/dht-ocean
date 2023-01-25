package svc

import (
	"context"
	"dht-ocean/common/util"
	"dht-ocean/ocean/internal/model"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"github.com/olivere/elastic/v7"
	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

const (
	emptyWaitTime = 5 * time.Second
)

type Indexer struct {
	ctx        context.Context
	cancel     context.CancelFunc
	svcCtx     *ServiceContext
	client     *elastic.Client
	waitTicker *time.Ticker
	hasTorrent chan struct{}
}

func NewIndexer(svcCtx *ServiceContext) *Indexer {
	indexer := &Indexer{
		svcCtx:     svcCtx,
		waitTicker: time.NewTicker(emptyWaitTime),
		hasTorrent: make(chan struct{}, 1),
	}
	indexer.ctx, indexer.cancel = context.WithCancel(context.Background())
	client, err := elastic.NewClient(
		elastic.SetURL(svcCtx.Config.ElasticSearch),
		elastic.SetSniff(false),
	)
	if err != nil {
		panic(err)
	}
	indexer.client = client
	return indexer
}

func (i *Indexer) Start() {
	for {
		select {
		case <-i.ctx.Done():
			return
		case <-i.hasTorrent:
			cnt := i.indexTorrents(100)
			if cnt > 0 {
				logx.Infof("Indexed %d torrents", cnt)
				i.waitTicker.Reset(emptyWaitTime)
				util.EmptyChannel(i.hasTorrent)
				i.hasTorrent <- struct{}{}
			} else {
				logx.Infof("Index done, wait for next tick")
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

func (i *Indexer) indexTorrents(limit int64) int {
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
		return 0
	}
	for _, record := range records {
		record.SearchUpdated = true
		err := i.saveToIndex(record)
		if err != nil {
			return 0
		}
	}
	logx.Infof("Indexed %d torrents.", len(records))
	return len(records)
}

func (i *Indexer) saveToIndex(torrent *model.Torrent) error {
	col := mgm.Coll(&model.Torrent{})
	_, err := i.client.Update().
		Index("torrents").
		Id(torrent.InfoHash).
		Doc(torrent).
		DocAsUpsert(true).
		Do(i.ctx)
	if err != nil {
		logx.Errorf("Failed to index torrent %s %s %v", torrent.InfoHash, torrent.Name, err)
		return err
	}
	_, err = col.UpdateByID(nil, torrent.InfoHash, bson.M{
		operator.Set: bson.M{
			"search_updated": true,
		},
	})
	if err != nil {
		logx.Errorf("Failed to update search_updated")
		return err
	}
	i.svcCtx.MetricOceanEvent.Inc("torrent_indexed")
	return nil
}

func (i *Indexer) CountTorrents() (int64, error) {
	cnt, err := i.client.Count("torrents").Do(i.ctx)
	return cnt, err
}
