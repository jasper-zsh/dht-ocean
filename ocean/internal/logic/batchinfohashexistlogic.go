package logic

import (
	"context"
	"encoding/hex"

	"dht-ocean/ocean/internal/svc"
	"dht-ocean/ocean/ocean"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
)

type BatchInfoHashExistLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBatchInfoHashExistLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchInfoHashExistLogic {
	return &BatchInfoHashExistLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *BatchInfoHashExistLogic) BatchInfoHashExist(in *ocean.BatchInfoHashExistRequest) (*ocean.BatchInfoHashExistResponse, error) {
	results := make([]bool, len(in.InfoHashes))
	for idx, infoHash := range in.InfoHashes {
		cnt, err := l.svcCtx.TorrentCollection.CountDocuments(nil, bson.M{
			"_id": hex.EncodeToString(infoHash),
		})
		if err != nil {
			return nil, err
		}
		results[idx] = cnt > 0
	}

	return &ocean.BatchInfoHashExistResponse{
		Results: results,
	}, nil
}
