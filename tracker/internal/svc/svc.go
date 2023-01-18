package svc

import (
	"dht-ocean/common/bittorrent/tracker"
	"dht-ocean/ocean/oceanclient"
	"dht-ocean/tracker/internal/config"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config   config.Config
	OceanRpc oceanclient.Ocean
	Tracker  tracker.Tracker
	Updater  *TrackerUpdater
}

func NewServiceContext(c config.Config) *ServiceContext {
	ctx := &ServiceContext{
		Config:   c,
		OceanRpc: oceanclient.NewOcean(zrpc.MustNewClient(c.Ocean)),
	}
	tr, err := tracker.NewUDPTracker(c.Tracker)
	if err != nil {
		logx.Errorf("Failed to create tracker. %v", err)
		panic(err)
	}
	ctx.Tracker = tr
	updater := NewTrackerUpdater(ctx, c.TrackerLimit)
	ctx.Updater = updater
	return ctx
}
