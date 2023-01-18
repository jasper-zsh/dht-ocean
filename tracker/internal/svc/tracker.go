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
	records := res.TorrentInfos

	hashes := make([][]byte, 0, len(records))
	for _, record := range records {
		hash, err := hex.DecodeString(record.InfoHash)
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
	for i, r := range scrapes {
		_, err := u.svcCtx.OceanRpc.UpdateTracker(context.Background(), &oceanclient.UpdateTrackerRequest{
			InfoHash: records[i].InfoHash,
			Seeders:  r.Seeders,
			Leechers: r.Leechers,
		})
		if err != nil {
			logx.Errorf("Failed to update tracker for %s", records[i].InfoHash)
		}
	}
}
