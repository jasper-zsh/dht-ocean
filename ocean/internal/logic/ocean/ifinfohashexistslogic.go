package oceanlogic

import (
	"context"

	"dht-ocean/ocean/internal/svc"
	"dht-ocean/ocean/ocean"

	"github.com/zeromicro/go-zero/core/logx"
)

type IfInfoHashExistsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewIfInfoHashExistsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IfInfoHashExistsLogic {
	return &IfInfoHashExistsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *IfInfoHashExistsLogic) IfInfoHashExists(in *ocean.IfInfoHashExistsRequest) (*ocean.IfInfoHashExistsResponse, error) {
	// todo: add your logic here and delete this line

	return &ocean.IfInfoHashExistsResponse{}, nil
}
