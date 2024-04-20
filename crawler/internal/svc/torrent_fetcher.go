package svc

import (
	"context"
	"dht-ocean/common/bittorrent"
	"dht-ocean/common/executor"
	"dht-ocean/common/model"
	"dht-ocean/common/util"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/juju/errors"
	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/threading"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/net/proxy"
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
	proxy         proxy.Dialer
	statsTicker   *time.Ticker
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
		statsTicker:   time.NewTicker(time.Second),
	}
	var err error
	if len(svcCtx.Config.Socks5Proxy) > 0 {
		f.proxy, err = proxy.SOCKS5("tcp", svcCtx.Config.Socks5Proxy, nil, nil)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	f.ctx, f.cancel = context.WithCancel(context.Background())

	_, err = os.Stat(svcCtx.Config.BloomFilterPath)
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
	routineGroup.Run(f.stats)

	routineGroup.Wait()
}

func (f *TorrentFetcher) Stop() {
	f.cancel()
	f.executor.Stop()
	tmpFilePath := f.svcCtx.Config.BloomFilterPath + ".tmp"
	logrus.Infof("Writing bloom filter to tmp file: %s", tmpFilePath)
	bloomFile, err := os.Create(tmpFilePath)
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
	err = os.Rename(tmpFilePath, f.svcCtx.Config.BloomFilterPath)
	if err != nil {
		logrus.Errorf("Failed to rename tmp bloom file: %+v", err)
		panic(err)
	}
}

func (f *TorrentFetcher) stats() {
	for {
		select {
		case <-f.ctx.Done():
			return
		case <-f.statsTicker.C:
			metricQueueSize.Set(float64(len(f.uncheckedChan)), "db_check_exists")
		}
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
	for {
		select {
		case <-f.ctx.Done():
			return
		case req := <-f.uncheckedChan:
			metricCrawlerEvent.Inc("db_exist_check")
			result := f.svcCtx.TorrentColl.FindOne(f.ctx, bson.M{
				"_id": hex.EncodeToString(req.InfoHash),
			}, &options.FindOneOptions{
				Projection: bson.M{
					"_id": true,
				},
			})
			err := result.Err()
			if err != nil {
				if err == mongo.ErrNoDocuments {
					f.commit(req)
				} else {
					logx.Errorf("Failed to check torrent exists: %+v", err)
				}
			} else {
				f.bloomFilter.Add(req.InfoHash)
			}
		}
	}
}

func (f *TorrentFetcher) commit(req TorrentRequest) {
	bt := bittorrent.NewBitTorrent(req.NodeID, req.InfoHash, req.Addr)
	bt.Proxy = f.proxy
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
		metricCrawlerEvent.Inc("connect_fail")
		return
	}
	defer bt.Stop()
	metadata, err := bt.GetMetadata()
	if err != nil {
		logx.Debugf("Failed to fetch metadata from %s %+v", bt.Addr, err)
		metricCrawlerEvent.Inc("metadata_fail")
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
		metricCrawlerEvent.Inc("decode_fail")
		return
	}
	logx.Debugf("Got torrent %s with %d files", torrent.Name, len(torrent.Files))
	if len(torrent.Name) > 0 {
		f.handleTorrent(torrent)
	}
}

func (f *TorrentFetcher) handleTorrent(torrent *bittorrent.Torrent) {
	t := model.NewTorrentFromBTTorrent(torrent)
	opts := options.UpdateOptions{}
	opts.SetUpsert(true)
	err := f.svcCtx.TorrentColl.Update(t, &opts)
	if err != nil {
		logx.Errorf("Failed to persist torrent %s : %+v", t.InfoHash, err)
		return
	}
	metricCrawlerEvent.Inc("torrent_upsert")
	raw, err := json.Marshal(t)
	if err != nil {
		logx.Errorf("Failed to marshal torrent: %+v", err)
		return
	}
	msg := message.NewMessage(watermill.NewUUID(), raw)
	err = f.svcCtx.Publisher.Publish(model.TopicNewTorrent, msg)
	if err != nil {
		logx.Errorf("Failed to publish new torrent %s %s message: %+v", hex.EncodeToString(torrent.InfoHash), torrent.Name, err)
		return
	}
	f.bloomFilter.Add(torrent.InfoHash)
	metricCrawlerEvent.Inc("new_torrent")
}
