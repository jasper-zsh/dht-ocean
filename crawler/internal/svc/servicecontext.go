package svc

import (
	"dht-ocean/crawler/internal/config"
	"dht-ocean/ocean/ocean"
	"dht-ocean/ocean/oceanclient"

	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config         config.Config
	OceanRpc       ocean.OceanClient
	Crawler        *Crawler
	TorrentFetcher *TorrentFetcher
}

func NewServiceContext(c config.Config) *ServiceContext {
	svcCtx := &ServiceContext{
		Config:   c,
		OceanRpc: oceanclient.NewOcean(zrpc.MustNewClient(c.Ocean)),
	}
	InjectCrawler(svcCtx)
	InjectTorrentFetcher(svcCtx)
	return svcCtx
}
