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
	updater, err := NewTrackerUpdater(ctx, svcCtx, c.TrackerLimit)
	if err != nil {
		logx.Errorf("Failed to initialize updater: %+v", err)
		panic(err)
	}
	svcCtx.Updater = updater
	return svcCtx
}
