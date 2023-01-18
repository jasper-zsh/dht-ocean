package svc

import (
	"dht-ocean/ocean/internal/config"
	"dht-ocean/ocean/internal/model"
	"github.com/kamva/mgm/v3"
	"github.com/sirupsen/logrus"
)

type ServiceContext struct {
	Config            config.Config
	TorrentCollection *mgm.Collection
	Indexer           *Indexer
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
	return svcCtx
}
