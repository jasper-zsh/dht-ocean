package svc

import (
	"dht-ocean/common/model"
	"dht-ocean/ocean/internal/config"

	"github.com/kamva/mgm/v3"
	"github.com/olivere/elastic/v7"
	"github.com/sirupsen/logrus"
	"github.com/zeromicro/go-zero/core/metric"
)

const (
	metricNamespace = "dht_ocean"
	metricSubsystem = "ocean"
)

type ServiceContext struct {
	Config            config.Config
	TorrentCollection *mgm.Collection
	MetricOceanEvent  metric.CounterVec
	TorrentCountGauge metric.GaugeVec
	ESClient          *elastic.Client
}

func NewServiceContext(c config.Config) *ServiceContext {
	err := model.InitMongo("dht_ocean", c.Mongo)
	if err != nil {
		logrus.Errorf("Failed to init MongoDB %v", err)
		panic(err)
	}
	svcCtx := &ServiceContext{
		Config:            c,
		TorrentCollection: mgm.Coll(&model.Torrent{}),
	}
	svcCtx.MetricOceanEvent = metric.NewCounterVec(&metric.CounterVecOpts{
		Namespace: metricNamespace,
		Subsystem: metricSubsystem,
		Name:      "ocean_event",
		Labels:    []string{"event"},
	})
	svcCtx.TorrentCountGauge = metric.NewGaugeVec(&metric.GaugeVecOpts{
		Namespace: metricNamespace,
		Subsystem: metricSubsystem,
		Name:      "torrent_count",
		Labels:    []string{"type"},
	})

	svcCtx.ESClient, err = elastic.NewClient(
		elastic.SetURL(svcCtx.Config.ElasticSearch),
		elastic.SetSniff(false),
	)
	if err != nil {
		panic(err)
	}
	return svcCtx
}
