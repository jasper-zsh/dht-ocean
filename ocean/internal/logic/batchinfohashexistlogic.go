package logic

import (
	"context"
	"encoding/hex"

	"dht-ocean/common/model"
	"dht-ocean/ocean/internal/svc"
	"dht-ocean/ocean/ocean"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/mongo"
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
		torrent := model.Torrent{}
		err := l.svcCtx.TorrentCollection.FindByID(hex.EncodeToString(infoHash), &torrent)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				results[idx] = false
			} else {
				return nil, err
			}
		} else {
			results[idx] = !torrent.Corrupted()
		}
	}

	return &ocean.BatchInfoHashExistResponse{
		Results: results,
	}, nil
}
