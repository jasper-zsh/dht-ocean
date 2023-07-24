package svc

import (
	"context"
	"dht-ocean/ocean/oceanclient"
	"encoding/hex"

	"github.com/sirupsen/logrus"
	"github.com/zeromicro/go-zero/core/logx"
)

type TrackerUpdater struct {
	svcCtx       *ServiceContext
	trackerLimit int64
}

func NewTrackerUpdater(svcCtx *ServiceContext, limit int64) *TrackerUpdater {
	r := &TrackerUpdater{
		svcCtx:       svcCtx,
		trackerLimit: limit,
	}
	return r
}

func (u *TrackerUpdater) Start() {
	for {
		u.refreshTracker()
	}
}

func (u *TrackerUpdater) Stop() {

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
	scrapes, err := u.svcCtx.Tracker.Scrape(hashes)
	if err != nil {
		logrus.Warnf("Failed to scrape %d torrents from tracker. %v", len(hashes), err)
		return
	}
	reqs := make([]*oceanclient.UpdateTrackerRequest, 0, len(scrapes))
	for i, r := range scrapes {
		reqs = append(reqs, &oceanclient.UpdateTrackerRequest{
			InfoHash: res.InfoHashes[i],
			Seeders:  r.Seeders,
			Leechers: r.Leechers,
		})
		_, err := u.svcCtx.OceanRpc.BatchUpdateTracker(context.Background(), &oceanclient.BatchUpdateTrackerRequest{
			Requests: reqs,
		})
		if err != nil {
			logx.Errorf("Failed to update tracker for %s", res.InfoHashes[i])
		}
	}
}
