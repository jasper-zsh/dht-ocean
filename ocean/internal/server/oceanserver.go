// Code generated by goctl. DO NOT EDIT!
// Source: ocean.proto

package server

import (
	"context"

	"dht-ocean/ocean/internal/logic"
	"dht-ocean/ocean/internal/svc"
	"dht-ocean/ocean/ocean"
)

type OceanServer struct {
	svcCtx *svc.ServiceContext
	ocean.UnimplementedOceanServer
}

func NewOceanServer(svcCtx *svc.ServiceContext) *OceanServer {
	return &OceanServer{
		svcCtx: svcCtx,
	}
}

func (s *OceanServer) IfInfoHashExists(ctx context.Context, in *ocean.IfInfoHashExistsRequest) (*ocean.IfInfoHashExistsResponse, error) {
	l := logic.NewIfInfoHashExistsLogic(ctx, s.svcCtx)
	return l.IfInfoHashExists(in)
}

func (s *OceanServer) BatchInfoHashExist(ctx context.Context, in *ocean.BatchInfoHashExistRequest) (*ocean.BatchInfoHashExistResponse, error) {
	l := logic.NewBatchInfoHashExistLogic(ctx, s.svcCtx)
	return l.BatchInfoHashExist(in)
}

func (s *OceanServer) CommitTorrent(ctx context.Context, in *ocean.CommitTorrentRequest) (*ocean.CommitTorrentResponse, error) {
	l := logic.NewCommitTorrentLogic(ctx, s.svcCtx)
	return l.CommitTorrent(in)
}

func (s *OceanServer) ListTorrentInfoForTracker(ctx context.Context, in *ocean.ListTorrentInfoForTrackerRequest) (*ocean.ListTorrentInfoForTrackerResponse, error) {
	l := logic.NewListTorrentInfoForTrackerLogic(ctx, s.svcCtx)
	return l.ListTorrentInfoForTracker(in)
}

func (s *OceanServer) UpdateTracker(ctx context.Context, in *ocean.UpdateTrackerRequest) (*ocean.UpdateTrackerResponse, error) {
	l := logic.NewUpdateTrackerLogic(ctx, s.svcCtx)
	return l.UpdateTracker(in)
}

func (s *OceanServer) BatchUpdateTracker(ctx context.Context, in *ocean.BatchUpdateTrackerRequest) (*ocean.BatchUpdateTrackerResponse, error) {
	l := logic.NewBatchUpdateTrackerLogic(ctx, s.svcCtx)
	return l.BatchUpdateTracker(in)
}
