package logic

import (
	"context"
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
	err := svc.UpdateTracker(l.ctx, in)
	if err != nil {
		return nil, err
	}

	return &ocean.UpdateTrackerResponse{}, nil
}
