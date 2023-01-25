package logic

import (
	"context"
	"dht-ocean/ocean/internal/model"
	"go.mongodb.org/mongo-driver/mongo/options"

	"dht-ocean/ocean/internal/svc"
	"dht-ocean/ocean/ocean"

	"github.com/zeromicro/go-zero/core/logx"
)

type CommitTorrentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCommitTorrentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CommitTorrentLogic {
	return &CommitTorrentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CommitTorrentLogic) CommitTorrent(in *ocean.CommitTorrentRequest) (*ocean.CommitTorrentResponse, error) {
	torrent := model.NewTorrentFromCommitRequest(in)
	opts := &options.UpdateOptions{}
	opts.SetUpsert(true)
	err := l.svcCtx.TorrentCollection.Update(torrent, opts)
	if err != nil {
		return nil, err
	}
	l.svcCtx.MetricOceanEvent.Inc("torrent_upsert")

	return &ocean.CommitTorrentResponse{}, nil
}
