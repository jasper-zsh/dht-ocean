package svc

import (
	"context"
	"dht-ocean/common/bittorrent"
	"dht-ocean/common/executor"
	"dht-ocean/common/util"
	"dht-ocean/ocean/ocean"
	"dht-ocean/ocean/oceanclient"
	"encoding/hex"
	"os"
	"reflect"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/threading"
)

const (
	targetCount = 1024 * 1024 * 100 // 100 million
)

type TorrentRequest struct {
	NodeID   []byte
	InfoHash []byte
	Addr     string
}

type TorrentFetcher struct {
	svcCtx        *ServiceContext
	ctx           context.Context
	cancel        context.CancelFunc
	uncheckedChan chan TorrentRequest
	bloomFilter   *util.BloomFilter
	executor      *executor.Executor[*bittorrent.BitTorrent]
}

func InjectTorrentFetcher(svcCtx *ServiceContext) {
	var err error
	svcCtx.TorrentFetcher, err = NewTorrentFetcher(svcCtx)
	if err != nil {
		logx.Errorf("Failed to initialize torrent fetcher %+v", err)
		panic(err)
	}
}

func NewTorrentFetcher(svcCtx *ServiceContext) (*TorrentFetcher, error) {
	f := &TorrentFetcher{
		uncheckedChan: make(chan TorrentRequest, 10000),
		svcCtx:        svcCtx,
	}
	f.ctx, f.cancel = context.WithCancel(context.Background())

	_, err := os.Stat(svcCtx.Config.BloomFilterPath)
	if err != nil && os.IsNotExist(err) {
		f.bloomFilter = util.NewBloomFilter(targetCount * 15)
	} else {
		bloomFile, err := os.Open(svcCtx.Config.BloomFilterPath)
		if err != nil {
			return nil, err
		}
		f.bloomFilter, err = util.LoadBloomFilter(bloomFile)
		if err != nil {
			return nil, err
		}
		err = bloomFile.Close()
		if err != nil {
			return nil, err
		}
	}

	f.executor = executor.NewExecutor(f.ctx, svcCtx.Config.TorrentWorkers, svcCtx.Config.TorrentMaxQueueSize, f.pullTorrent)

	return f, nil
}

func (f *TorrentFetcher) Start() {
	routineGroup := threading.NewRoutineGroup()

	routineGroup.RunSafe(f.checkExistLoop)
	routineGroup.RunSafe(f.executor.Start)

	routineGroup.Wait()
}

func (f *TorrentFetcher) Stop() {
	f.cancel()
	f.executor.Stop()
	bloomFile, err := os.Create(f.svcCtx.Config.BloomFilterPath)
	if err != nil {
		panic(err)
	}
	err = f.bloomFilter.Save(bloomFile)
	if err != nil {
		panic(err)
	}
	err = bloomFile.Close()
	if err != nil {
		panic(err)
	}
}

func (f *TorrentFetcher) Push(req TorrentRequest) {
	exist := f.bloomFilter.Exists(req.InfoHash)
	if exist {
		f.uncheckedChan <- req
	} else {
		f.commit(req)
	}
}

func (f *TorrentFetcher) checkExistLoop() {
	batchSize := f.svcCtx.Config.CheckExistBatchSize
	buf := make([]TorrentRequest, 0, batchSize)
	for {
		select {
		case <-f.ctx.Done():
			return
		case req := <-f.uncheckedChan:
			buf = append(buf, req)
			if len(buf) > 0 {
				go f.batchCheck(buf)
				buf = make([]TorrentRequest, 0, batchSize)
			}
		}
	}
}

func (f *TorrentFetcher) batchCheck(reqs []TorrentRequest) {
	infoHashes := make([][]byte, 0, len(reqs))
	for _, req := range reqs {
		infoHashes = append(infoHashes, req.InfoHash)
	}
	req := &oceanclient.BatchInfoHashExistRequest{
		InfoHashes: infoHashes,
	}
	res, err := f.svcCtx.OceanRpc.BatchInfoHashExist(f.ctx, req)
	if err != nil {
		logx.Errorf("Failed to batch check %d info hash exists. %+v", len(reqs), err)
		return
	}
	for idx, result := range res.Results {
		if result {
			f.bloomFilter.Add(infoHashes[idx])
		} else {
			f.commit(reqs[idx])
		}
	}
}

func (f *TorrentFetcher) commit(req TorrentRequest) {
	bt := bittorrent.NewBitTorrent(req.NodeID, req.InfoHash, req.Addr)
	bt.SetTrafficMetricFunc(func(label string, length int) {
		metricTrafficCounter.Add(float64(length), label)
	})
	if f.executor.QueueSize() >= f.svcCtx.Config.TorrentMaxQueueSize {
		metricCrawlerEvent.Inc("drop_torrent")
		logx.Debugf("Pull torrent task queue full, drop %s", hex.EncodeToString(req.InfoHash))
	} else {
		f.executor.Commit(bt)
	}
}

func (f *TorrentFetcher) pullTorrent(bt *bittorrent.BitTorrent) {
	metricCrawlerEvent.Inc("pull_torrent")
	err := bt.Start()
	if err != nil {
		logx.Debugf("Failed to connect to peer to fetch metadata from %s %v", bt.Addr, err)
		return
	}
	defer bt.Stop()
	metadata, err := bt.GetMetadata()
	if err != nil {
		logx.Debugf("Failed to fetch metadata from %s %+v", bt.Addr, err)
		return
	}
	torrent := &bittorrent.Torrent{
		InfoHash: bt.InfoHash,
	}
	decoder, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		Result:           torrent,
		DecodeHook: func(src reflect.Kind, target reflect.Kind, from interface{}) (interface{}, error) {
			if target == reflect.String {
				switch v := from.(type) {
				case []byte:
					return strings.ToValidUTF8(string(v), ""), nil
				case string:
					return strings.ToValidUTF8(v, ""), nil
				}

			}
			return from, nil
		},
	})
	err = decoder.Decode(metadata)
	if err != nil {
		logx.Errorf("Failed to decode metadata %v %v", metadata, err)
		return
	}
	logx.Debugf("Got torrent %s with %d files", torrent.Name, len(torrent.Files))
	if len(torrent.Name) > 0 {
		f.handleTorrent(torrent)
	}
}

func (f *TorrentFetcher) handleTorrent(torrent *bittorrent.Torrent) {
	req := &ocean.CommitTorrentRequest{
		InfoHash:  torrent.InfoHash,
		Name:      torrent.Name,
		Publisher: torrent.Publisher,
		Source:    torrent.Source,
		Files:     make([]*ocean.File, 0, len(torrent.Files)),
	}
	for _, file := range torrent.Files {
		req.Files = append(req.Files, &ocean.File{
			Length:   file.Length,
			Paths:    file.Path,
			FileHash: file.FileHash,
		})
	}
	_, err := f.svcCtx.OceanRpc.CommitTorrent(context.TODO(), req)
	if err != nil {
		logx.Errorf("Failed to commit torrent %s %s. %v", torrent.InfoHash, torrent.Name, err)
		return
	}
	f.bloomFilter.Add(req.InfoHash)
}
