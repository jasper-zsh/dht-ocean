package logic

import (
	"context"
	"encoding/hex"
	"go.mongodb.org/mongo-driver/bson"

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
	cnt, err := l.svcCtx.TorrentCollection.CountDocuments(nil, bson.M{
		"_id": hex.EncodeToString(in.InfoHash),
	})
	if err != nil {
		return nil, err
	}

	return &ocean.IfInfoHashExistsResponse{
		Exists: cnt > 0,
	}, nil
}
