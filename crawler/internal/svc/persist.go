package svc

import (
	"context"
	"dht-ocean/common/model"
	"encoding/json"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-amqp/v2/pkg/amqp"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/juju/errors"
	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type DBPersist struct {
	svcCtx *ServiceContext
	ctx    context.Context

	subscriber message.Subscriber
	router     *message.Router
}

const (
	handlerNameDBPersist = "db_persist"
)

func NewDBPersist(svcCtx *ServiceContext, ctx context.Context) (*DBPersist, error) {
	amqpConfig := amqp.NewDurablePubSubConfig(svcCtx.Config.AMQP, amqp.GenerateQueueNameConstant("db_persist"))
	amqpConfig.Consume.Qos.PrefetchCount = svcCtx.Config.AMQPPreFetch
	subscriber, err := amqp.NewSubscriber(amqpConfig, watermill.NewStdLogger(false, false))
	if err != nil {
		return nil, errors.Trace(err)
	}
	err = subscriber.SubscribeInitialize(model.TopicTrackerUpdated)
	if err != nil {
		return nil, errors.Trace(err)
	}
	router, err := message.NewRouter(message.RouterConfig{}, watermill.NewStdLogger(false, false))
	if err != nil {
		return nil, errors.Trace(err)
	}
	ret := &DBPersist{
		svcCtx:     svcCtx,
		ctx:        ctx,
		subscriber: subscriber,
		router:     router,
	}
	router.AddNoPublisherHandler(handlerNameDBPersist, model.TopicNewTorrent, subscriber, ret.consumeNewTorrent)
	return ret, nil
}

func (p *DBPersist) consumeNewTorrent(msg *message.Message) error {
	t := model.Torrent{}
	err := json.Unmarshal(msg.Payload, &t)
	if err != nil {
		return errors.Trace(err)
	}
	opts := &options.UpdateOptions{}
	opts.SetUpsert(true)
	err = p.svcCtx.TorrentColl.Update(&t, opts)
	if err != nil {
		return errors.Trace(err)
	}
	metricCrawlerEvent.Inc("torrent_upsert")
	return nil
}

func (p *DBPersist) Start() {
	err := p.router.Run(p.ctx)
	if err != nil {
		logx.Errorf("Router error: %+v", err)
	}
}

func (p *DBPersist) Stop() {
	p.router.Close()
}
