package svc

import (
	"context"
	"dht-ocean/common/model"
	"dht-ocean/crawler/internal/config"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-amqp/v2/pkg/amqp"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/kamva/mgm/v3"
	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceContext struct {
	Config         config.Config
	Crawler        *Crawler
	TorrentFetcher *TorrentFetcher
	Publisher      message.Publisher
	TorrentColl    *mgm.Collection
	DBPersist      *DBPersist
}

func NewServiceContext(c config.Config, ctx context.Context) *ServiceContext {
	svcCtx := &ServiceContext{
		Config: c,
	}
	amqpConfig := amqp.NewDurablePubSubConfig(c.AMQP, amqp.GenerateQueueNameTopicName)
	amqpConfig.Consume.Qos.PrefetchCount = svcCtx.Config.AMQPPreFetch
	publisher, err := amqp.NewPublisher(amqpConfig, watermill.NewStdLogger(false, false))
	if err != nil {
		logx.Errorf("Failed to initialize amqp: %+v", err)
		panic(err)
	}
	svcCtx.Publisher = publisher

	err = model.InitMongo("dht_ocean", c.Mongo)
	if err != nil {
		logx.Errorf("Failed to initialize MongoDB: %+v", err)
		panic(err)
	}
	svcCtx.TorrentColl = mgm.Coll(&model.Torrent{})

	svcCtx.DBPersist, err = NewDBPersist(svcCtx, ctx)
	if err != nil {
		logx.Errorf("Failed to initialize db persist: %+v", err)
		panic(err)
	}

	InjectCrawler(svcCtx)
	InjectTorrentFetcher(svcCtx)
	return svcCtx
}
