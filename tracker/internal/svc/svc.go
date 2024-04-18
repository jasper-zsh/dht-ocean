package svc

import (
	"context"
	"dht-ocean/common/bittorrent/tracker"
	"dht-ocean/tracker/internal/config"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceContext struct {
	Config  config.Config
	Tracker tracker.Tracker
	Updater *TrackerUpdater
}

func NewServiceContext(ctx context.Context, c config.Config) *ServiceContext {
	svcCtx := &ServiceContext{
		Config: c,
	}
	tr, err := tracker.NewUDPTracker(ctx, c.Tracker)
	if err != nil {
		logx.Errorf("Failed to create tracker. %v", err)
		panic(err)
	}
	svcCtx.Tracker = tr
	updater := NewTrackerUpdater(ctx, svcCtx, c.TrackerLimit)
	svcCtx.Updater = updater
	return svcCtx
}
