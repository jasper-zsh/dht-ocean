package logic

import (
	"context"
	"dht-ocean/ocean/internal/model"
	"dht-ocean/ocean/internal/svc"
	"dht-ocean/ocean/ocean"
	"github.com/kamva/mgm/v3/operator"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListTorrentInfoForTrackerLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListTorrentInfoForTrackerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTorrentInfoForTrackerLogic {
	return &ListTorrentInfoForTrackerLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListTorrentInfoForTrackerLogic) ListTorrentInfoForTracker(in *ocean.ListTorrentInfoForTrackerRequest) (*ocean.ListTorrentInfoForTrackerResponse, error) {
	records, err := l.getRecords(in)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	wait := sync.WaitGroup{}
	for _, r := range records {
		wait.Add(1)
		infoHash := r.InfoHash
		go func() {
			defer wait.Done()
			_, err := l.svcCtx.TorrentCollection.UpdateByID(l.ctx, infoHash, bson.M{
				operator.Set: bson.M{
					"tracker_last_tried_at": now,
				},
			})
			if err != nil {
				logx.Errorf("Failed to update tracker last tries time for %s", r.InfoHash)
			}
		}()
	}
	wait.Wait()
	res := &ocean.ListTorrentInfoForTrackerResponse{
		TorrentInfos: make([]*ocean.Torrent, 0, len(records)),
	}
	for _, record := range records {
		res.TorrentInfos = append(res.TorrentInfos, &ocean.Torrent{
			InfoHash: record.InfoHash,
		})
	}

	return res, nil
}

func (l *ListTorrentInfoForTrackerLogic) getRecords(in *ocean.ListTorrentInfoForTrackerRequest) ([]*model.Torrent, error) {
	records := make([]*model.Torrent, 0)
	notTried := make([]*model.Torrent, 0)
	err := l.svcCtx.TorrentCollection.SimpleFind(&notTried, bson.M{
		"tracker_last_tried_at": bson.M{
			operator.Eq: nil,
		},
	}, &options.FindOptions{
		Sort: bson.M{
			"updated_at": 1,
		},
		Limit: &in.Size,
	})
	if err != nil {
		logx.Errorf("Failed to load torrents for tracker. %v", err)
		return nil, err
	}
	records = append(records, notTried...)
	limit := in.Size - int64(len(notTried))
	if limit == 0 {
		return records, nil
	}
	newRecords := make([]*model.Torrent, 0)
	err = l.svcCtx.TorrentCollection.SimpleFind(&newRecords, bson.M{
		"tracker_updated_at": bson.M{
			operator.Eq: nil,
		},
	}, &options.FindOptions{
		Sort: bson.M{
			"tracker_last_tried_at": 1,
		},
		Limit: &limit,
	})
	if err != nil {
		logrus.Errorf("Failed to load torrents for tracker. %v", err)
		return nil, err
	}
	records = append(records, newRecords...)
	limit = limit - int64(len(newRecords))
	if limit == 0 {
		return records, nil
	}
	outdated := make([]*model.Torrent, 0)
	age := time.Now().Add(-6 * time.Hour)
	err = l.svcCtx.TorrentCollection.SimpleFind(&outdated, bson.M{
		"tracker_updated_at": bson.M{
			operator.Lte: age,
		},
	}, &options.FindOptions{
		Sort: bson.M{
			"tracker_updated_at": 1,
		},
		Limit: &limit,
	})
	if err != nil {
		logrus.Errorf("Failed to load torrents for tracker. %v", err)
		return nil, err
	}
	records = append(records, outdated...)
	return records, nil
}
