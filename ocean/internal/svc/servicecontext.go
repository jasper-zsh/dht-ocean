package svc

import (
	"dht-ocean/ocean/internal/config"
	"dht-ocean/ocean/internal/model"
	"github.com/kamva/mgm/v3"
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
	Indexer           *Indexer
	MetricOceanEvent  metric.CounterVec
	TorrentCountGauge metric.GaugeVec
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
	svcCtx.Indexer = NewIndexer(svcCtx)
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
	return svcCtx
}
