package logic

import (
	"context"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
	"time"

	"dht-ocean/ocean/internal/svc"
	"dht-ocean/ocean/ocean"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateTrackerLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateTrackerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateTrackerLogic {
	return &UpdateTrackerLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateTrackerLogic) UpdateTracker(in *ocean.UpdateTrackerRequest) (*ocean.UpdateTrackerResponse, error) {
	now := time.Now()
	_, err := l.svcCtx.TorrentCollection.UpdateByID(l.ctx, in.InfoHash, bson.M{
		operator.Set: bson.M{
			"seeders":            in.Seeders,
			"leechers":           in.Leechers,
			"tracker_updated_at": now,
			"updated_at":         now,
			"search_updated":     false,
		},
	})
	if err != nil {
		return nil, err
	}

	return &ocean.UpdateTrackerResponse{}, nil
}
