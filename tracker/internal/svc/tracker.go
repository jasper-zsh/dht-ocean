package svc

import (
	"context"
	"dht-ocean/ocean/oceanclient"
	"encoding/hex"

	"github.com/sirupsen/logrus"
	"github.com/zeromicro/go-zero/core/logx"
)

type TrackerUpdater struct {
	ctx          context.Context
	cancel       context.CancelFunc
	svcCtx       *ServiceContext
	trackerLimit int64
}

func NewTrackerUpdater(ctx context.Context, svcCtx *ServiceContext, limit int64) *TrackerUpdater {
	r := &TrackerUpdater{
		svcCtx:       svcCtx,
		trackerLimit: limit,
	}
	r.ctx, r.cancel = context.WithCancel(ctx)
	return r
}

func (u *TrackerUpdater) Start() {
	go u.handleResult()
	u.fetch()
}

func (u *TrackerUpdater) Stop() {
	u.cancel()
}

func (u *TrackerUpdater) fetch() {
	for {
		select {
		case <-u.ctx.Done():
			return
		default:
			u.refreshTracker()
		}
	}
}

func (u *TrackerUpdater) handleResult() {
	for {
		select {
		case <-u.ctx.Done():
			return
		case result := <-u.svcCtx.Tracker.Result():
			reqs := make([]*oceanclient.UpdateTrackerRequest, 0, len(result))
			for _, r := range result {
				reqs = append(reqs, &oceanclient.UpdateTrackerRequest{
					InfoHash: hex.EncodeToString(r.InfoHash),
					Seeders:  r.Seeders,
					Leechers: r.Leechers,
				})
			}
			_, err := u.svcCtx.OceanRpc.BatchUpdateTracker(context.Background(), &oceanclient.BatchUpdateTrackerRequest{
				Requests: reqs,
			})
			if err != nil {
				logx.Errorf("Failed to update tracker: %+v", err)
			}
		}
	}
}

func (u *TrackerUpdater) refreshTracker() {
	res, err := u.svcCtx.OceanRpc.ListTorrentInfoForTracker(context.Background(), &oceanclient.ListTorrentInfoForTrackerRequest{
		Size: u.trackerLimit,
	})
	if err != nil {
		logx.Errorf("Failed to fetch torrent infos for tracker from ocean. %v", err)
		return
	}

	hashes := make([][]byte, 0, len(res.InfoHashes))
	for _, record := range res.InfoHashes {
		hash, err := hex.DecodeString(record)
		if err != nil {
			logrus.Errorf("broken torrent record, skip tracker scrape")
			return
		}
		hashes = append(hashes, hash)
	}
	err = u.svcCtx.Tracker.Scrape(hashes)
	if err != nil {
		logrus.Warnf("Failed to scrape %d torrents from tracker. %v", len(hashes), err)
		return
	}
}
