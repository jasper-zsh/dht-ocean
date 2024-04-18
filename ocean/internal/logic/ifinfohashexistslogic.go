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
	ret := &ocean.IfInfoHashExistsResponse{}
	torrent := model.Torrent{}
	err := l.svcCtx.TorrentCollection.FindByID(hex.EncodeToString(in.InfoHash), &torrent)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			ret.Exists = false
		} else {
			return nil, err
		}
	} else {
		ret.Exists = !torrent.Corrupted()
	}

	return ret, nil
}
