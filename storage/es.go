package storage

import (
	"context"
	"dht-ocean/model"
	"github.com/olivere/elastic/v7"
	"github.com/sirupsen/logrus"
)

var _ TorrentStorage = (*ESTorrentStorage)(nil)

type ESTorrentStorage struct {
	ctx    context.Context
	client *elastic.Client
}

func NewESTorrentStorage(host string) (*ESTorrentStorage, error) {
	h := &ESTorrentStorage{}
	h.ctx = context.Background()
	client, err := elastic.NewClient(
		elastic.SetURL(host),
		elastic.SetSniff(false),
	)
	if err != nil {
		return nil, err
	}
	h.client = client
	return h, nil
}

func (h *ESTorrentStorage) Store(torrent *model.Torrent) {
	_, err := h.client.Update().Index("torrents").Id(torrent.InfoHash).Upsert(torrent).Do(h.ctx)
	if err != nil {
		logrus.Errorf("Failed to index torrent %s %s %v", torrent.InfoHash, torrent.Name, err)
	}
}
