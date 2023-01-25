package logic

import (
	"context"

	"dht-ocean/ocean/internal/svc"
	"dht-ocean/ocean/ocean"

	"github.com/zeromicro/go-zero/core/logx"
)

type BatchUpdateTrackerLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBatchUpdateTrackerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchUpdateTrackerLogic {
	return &BatchUpdateTrackerLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *BatchUpdateTrackerLogic) BatchUpdateTracker(in *ocean.BatchUpdateTrackerRequest) (*ocean.BatchUpdateTrackerResponse, error) {
	svc.BatchUpdateTracker(l.ctx, l.svcCtx, in.Requests)

	return &ocean.BatchUpdateTrackerResponse{}, nil
}
