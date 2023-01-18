package oceanlogic

import (
	"context"

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
	// todo: add your logic here and delete this line

	return &ocean.CommitTorrentResponse{}, nil
}
