package svc

import (
	"context"
	"dht-ocean/ocean/internal/model"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"github.com/olivere/elastic/v7"
	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

type Indexer struct {
	ctx    context.Context
	client *elastic.Client
}

func NewIndexer(svcCtx *ServiceContext) *Indexer {
	indexer := &Indexer{
		ctx: context.Background(),
	}
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
		cnt := i.indexTorrents(100)
		logx.Infof("Indexed %d torrents", cnt)
		if cnt == 0 {
			time.Sleep(5 * time.Second)
		}
	}
}

func (i *Indexer) Stop() {

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
		_, err = col.UpdateByID(nil, record.InfoHash, bson.M{
			operator.Set: bson.M{
				"search_updated": true,
			},
		})
		if err != nil {
			logx.Errorf("Failed to update search_updated")
			return 0
		}
	}
	logx.Infof("Indexed %d torrents.", len(records))
	return len(records)
}

func (i *Indexer) saveToIndex(torrent *model.Torrent) error {
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
	return nil
}
