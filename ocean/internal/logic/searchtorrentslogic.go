package logic

import (
	"context"
	"encoding/json"

	"dht-ocean/ocean/internal/model"
	"dht-ocean/ocean/internal/svc"
	"dht-ocean/ocean/ocean"

	"github.com/olivere/elastic/v7"
	"github.com/zeromicro/go-zero/core/logx"
)

type SearchTorrentsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSearchTorrentsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchTorrentsLogic {
	return &SearchTorrentsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SearchTorrentsLogic) SearchTorrents(in *ocean.SearchTorrentsRequest) (*ocean.TorrentPageResponse, error) {
	q := elastic.NewBoolQuery().Should(
		elastic.NewMatchQuery("name", in.Keyword).Boost(3),
		elastic.NewMatchQuery("tags", in.Keyword),
		elastic.NewMatchQuery("files.paths", in.Keyword).Boost(2),
	)
	req := l.svcCtx.ESClient.Search("torrents").
		Size(int(in.PerPage)).
		From(int(in.PerPage * (in.Page - 1))).
		Query(q)
	for _, sort := range in.SortParams {
		req.Sort(sort.Field, sort.Asc)
	}
	res, err := req.Do(l.ctx)
	if err != nil {
		return nil, err
	}
	torrents := make([]*ocean.Torrent, 0, len(res.Hits.Hits))
	for _, hit := range res.Hits.Hits {
		var t model.Torrent
		err := json.Unmarshal(hit.Source, &t)
		if err != nil {
			return nil, err
		}
		files := make([]*ocean.File, 0, len(t.Files))
		for _, f := range t.Files {
			files = append(files, &ocean.File{
				Length:   f.Length,
				Paths:    f.Paths,
				FileHash: []byte(f.FileHash),
			})
		}
		torrent := &ocean.Torrent{
			InfoHash: t.InfoHash,
			Name:     t.Name,
			Files:    files,
		}
		if t.Seeders != nil {
			torrent.Seeders = *t.Seeders
		}
		if t.Leechers != nil {
			torrent.Leechers = *t.Leechers
		}
		torrents = append(torrents, torrent)
	}

	return &ocean.TorrentPageResponse{
		Torrents: torrents,
		Total:    uint32(res.TotalHits()),
	}, nil
}
