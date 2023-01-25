package svc

import (
	"context"
	"github.com/zeromicro/go-zero/core/logx"
	"time"
)

type Stats struct {
	ctx    context.Context
	cancel context.CancelFunc
	ticker *time.Ticker
	svcCtx *ServiceContext
}

func NewStats(svcCtx *ServiceContext) *Stats {
	ret := &Stats{
		ticker: time.NewTicker(30 * time.Second),
		svcCtx: svcCtx,
	}
	ret.ctx, ret.cancel = context.WithCancel(context.Background())
	return ret
}

func (s *Stats) Start() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.ticker.C:
			s.stats()
		}
	}
}

func (s *Stats) stats() {
	cnt, err := s.svcCtx.TorrentCollection.EstimatedDocumentCount(s.ctx)
	if err != nil {
		logx.Errorf("Failed to count torrents. %v", err)
	} else {
		s.svcCtx.TorrentCountGauge.Set(float64(cnt), "total")
	}
	cnt, err = s.svcCtx.Indexer.CountTorrents()
	if err != nil {
		logx.Errorf("Failed to count indexed torrents. %v", err)
	} else {
		s.svcCtx.TorrentCountGauge.Set(float64(cnt), "indexed")
	}
}

func (s *Stats) Stop() {
	s.ticker.Stop()
	s.cancel()
}
