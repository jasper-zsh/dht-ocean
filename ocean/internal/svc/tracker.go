package svc

import (
	"context"
	"dht-ocean/ocean/internal/model"
	"dht-ocean/ocean/ocean"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
	"sync"
	"time"
)

func UpdateTracker(ctx context.Context, svcCtx *ServiceContext, in *ocean.UpdateTrackerRequest) error {
	now := time.Now()
	coll := mgm.Coll(&model.Torrent{})
	_, err := coll.UpdateByID(ctx, in.InfoHash, bson.M{
		operator.Set: bson.M{
			"seeders":            in.Seeders,
			"leechers":           in.Leechers,
			"tracker_updated_at": now,
			"updated_at":         now,
			"search_updated":     false,
		},
	})
	if err != nil {
		logx.Errorf("Failed to update tracker for %s", in.InfoHash)
		return err
	}
	svcCtx.MetricOceanEvent.Inc("tracker_updated")
	return nil
}

func BatchUpdateTracker(ctx context.Context, svcCtx *ServiceContext, reqs []*ocean.UpdateTrackerRequest) {
	group := sync.WaitGroup{}
	for _, req := range reqs {
		group.Add(1)
		req := req
		go func() {
			defer group.Done()
			_ = UpdateTracker(ctx, svcCtx, req)
		}()
	}
	group.Wait()
}
