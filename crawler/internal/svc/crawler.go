package svc

import (
	"bytes"
	"context"
	"dht-ocean/common/bencode"
	"dht-ocean/common/bittorrent"
	"dht-ocean/common/dht"
	"dht-ocean/common/executor"
	"dht-ocean/ocean/ocean"
	"dht-ocean/ocean/oceanclient"
	"encoding/hex"
	"fmt"
	"net"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/zeromicro/go-zero/core/bloom"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/metric"
	"github.com/zeromicro/go-zero/core/threading"
	"golang.org/x/time/rate"
)

const (
	metricsNamespace = "dht_ocean"
	metricsSubsystem = "crawler"
)

type InfoHashFilter func(infoHash []byte) bool

type Crawler struct {
	ctx                     context.Context
	cancel                  context.CancelFunc
	addr                    *net.UDPAddr
	conn                    *net.UDPConn
	nodeID                  []byte
	bootstrapNodes          []*dht.Node
	neighbours              chan *dht.Node
	findNodeLimiter         *rate.Limiter
	ticker                  *time.Ticker
	tm                      *dht.TransactionManager
	packetBuffers           chan *dht.Packet
	maxQueueSize            int
	svcCtx                  *ServiceContext
	bloomFilter             *bloom.Filter
	executor                *executor.Executor[*bittorrent.BitTorrent]
	metricDHTSendCounter    metric.CounterVec
	metricDHTReceiveCounter metric.CounterVec
	metricTrafficCounter    metric.CounterVec
	metricCrawlerEvent      metric.CounterVec
	metricQueueSize         metric.GaugeVec
}

func InjectCrawler(svcCtx *ServiceContext) {
	crawler, err := NewCrawler(svcCtx)
	if err != nil {
		logx.Errorf("Failed to initialize crawler. %v", err)
		panic(err)
	}
	svcCtx.Crawler = crawler
}

func NewCrawler(svcCtx *ServiceContext) (*Crawler, error) {
	var nodeID []byte
	var err error
	if len(svcCtx.Config.NodeID) > 0 {
		nodeID, err = hex.DecodeString(svcCtx.Config.NodeID)
		if err != nil {
			logx.Errorf("Failed to decode Node ID: %s %v", svcCtx.Config.NodeID, err)
			nodeID = nil
		}
	} else {
		nodeID = dht.GenerateNodeID()
		logx.Infof("Node ID not set, generated: %s", hex.EncodeToString(nodeID))
	}

	addr, err := net.ResolveUDPAddr("udp", svcCtx.Config.DHTListen)
	if err != nil {
		logx.Errorf("Failed to resolve listen address %s. %v", svcCtx.Config.DHTListen, err)
		return nil, err
	}

	c := &Crawler{
		addr:          addr,
		nodeID:        nodeID,
		tm:            &dht.TransactionManager{},
		packetBuffers: make(chan *dht.Packet, 1000),
		maxQueueSize:  svcCtx.Config.MaxQueueSize,
		svcCtx:        svcCtx,
		neighbours:    make(chan *dht.Node, svcCtx.Config.MaxQueueSize),
	}
	redis := c.svcCtx.Config.Redis.NewRedis()
	c.bloomFilter = bloom.New(redis, "torrent_bloom", 1024*1024*5)
	c.SetBootstrapNodes(svcCtx.Config.BootstrapNodes)
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.executor = executor.NewExecutor(c.ctx, svcCtx.Config.TorrentWorkers, svcCtx.Config.TorrentMaxQueueSize, c.pullTorrent)
	c.metricDHTSendCounter = metric.NewCounterVec(&metric.CounterVecOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "dht_send",
		Labels:    []string{"type"},
	})
	c.metricDHTReceiveCounter = metric.NewCounterVec(&metric.CounterVecOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "dht_receive",
		Labels:    []string{"type"},
	})
	c.metricTrafficCounter = metric.NewCounterVec(&metric.CounterVecOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "traffic",
		Labels:    []string{"type"},
	})
	c.metricQueueSize = metric.NewGaugeVec(&metric.GaugeVecOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "neighbours_queue_size",
	})
	c.metricCrawlerEvent = metric.NewCounterVec(&metric.CounterVecOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "crawler_event",
		Labels:    []string{"event"},
	})
	c.findNodeLimiter = rate.NewLimiter(rate.Limit(c.svcCtx.Config.FindNodeRateLimit), c.svcCtx.Config.FindNodeRateLimit)
	return c, nil
}

func (c *Crawler) SetMaxQueueSize(size int) {
	c.maxQueueSize = size
}

func (c *Crawler) SetBootstrapNodes(addrs []string) {
	nodes := make([]*dht.Node, 0, len(addrs))
	group := sync.WaitGroup{}
	for _, strAddr := range addrs {
		group.Add(1)
		strAddr := strAddr
		go func() {
			defer group.Done()
			addr, err := net.ResolveUDPAddr("udp4", strAddr)
			if err != nil {
				logx.Errorf("Failed to resolve bootstrap node address %s. %v", strAddr, err)
				return
			}
			node := &dht.Node{
				NodeID: c.nodeID,
				Addr:   addr,
			}
			nodes = append(nodes, node)
		}()
	}
	group.Wait()
	c.bootstrapNodes = nodes
}

func (c *Crawler) bootstrap() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.ticker.C:
			c.metricQueueSize.Set(float64(len(c.neighbours)))
			if len(c.neighbours) == 0 {
				logx.Infof("Running out of neighbours, sending %d bootstrap nodes", len(c.bootstrapNodes))
				for _, node := range c.bootstrapNodes {
					c.neighbours <- node
				}
			}
		}
	}
}

func (c *Crawler) Start() {
	conn, err := net.ListenUDP("udp", c.addr)
	if err != nil {
		logx.Errorf("Failed to listen udp on %s. %v", c.addr, err)
		panic(err)
	}
	c.conn = conn

	c.ticker = time.NewTicker(time.Second)
	routineGroup := threading.NewRoutineGroup()
	routineGroup.RunSafe(c.listen)
	routineGroup.RunSafe(c.handleMessage)
	routineGroup.RunSafe(c.makeNeighbours)
	routineGroup.RunSafe(c.bootstrap)
	routineGroup.RunSafe(c.executor.Start)
	routineGroup.Wait()
}

func (c *Crawler) Stop() {
	if c.conn != nil {
		_ = c.conn.Close()
	}
	if c.ticker != nil {
		c.ticker.Stop()
	}
	c.executor.Stop()
	c.cancel()
}

func (c *Crawler) listen() {
	buf := make([]byte, 65536)
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			transfered, addr, err := c.conn.ReadFromUDP(buf)
			if err != nil {
				continue
			}
			logx.Debugf("Read %d bytes from udp %s", transfered, addr)
			msg := make([]byte, transfered)
			copy(msg, buf)
			pkt := dht.NewPacketFromBuffer(msg)
			pkt.Addr = addr
			c.packetBuffers <- pkt
		}
	}
}

func (c *Crawler) handleMessage() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case pkt := <-c.packetBuffers:
			err := pkt.Decode()
			if err != nil {
				logx.Debugf("Failed to parse DHT packet. %s %v", pkt, err)
				continue
			}
			c.onMessage(pkt, pkt.Addr)
		}
	}
}

func (c *Crawler) sendPacket(pkt *dht.Packet, addr *net.UDPAddr) error {
	encoded, err := pkt.Encode()
	if err != nil {
		logx.Errorf("Failed to encode packet: %s, %v", pkt, err)
		return err
	}
	bytes, err := c.conn.WriteToUDP([]byte(encoded), addr)
	if err != nil {
		logx.Errorf("Failed to write to udp %s %v", addr, err)
		return err
	}
	logx.Debugf("Send %d bytes to %s", bytes, addr)
	return nil
}

func (c *Crawler) makeNeighbours() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case node := <-c.neighbours:
			_ = c.findNodeLimiter.Wait(c.ctx)
			c.sendFindNode(dht.GetNeighbourID(node.NodeID, c.nodeID), dht.GenerateNodeID(), node.Addr)
		}
	}
}

func (c *Crawler) sendFindNode(nodeID []byte, target []byte, addr *net.UDPAddr) {
	//req := dht.NewFindNodeRequest(dht.GetNeighbourID(nodeID, c.nodeID), target)
	req := dht.NewFindNodeRequest(nodeID, target)
	req.SetT(c.tm.NextTransactionID())
	_ = c.sendPacket(req.Packet, addr)
	c.metricDHTSendCounter.Inc("find_node")
	c.metricTrafficCounter.Add(float64(req.Packet.Size()), "out_find_node")
}

func (c *Crawler) sendPing(addr *net.UDPAddr) {
	req := dht.NewPingRequest(c.nodeID)
	req.SetT(c.tm.NextTransactionID())
	_ = c.sendPacket(req.Packet, addr)
	c.metricDHTSendCounter.Inc("ping")
	c.metricTrafficCounter.Add(float64(req.Packet.Size()), "out_ping")
}

func (c *Crawler) onMessage(packet *dht.Packet, addr *net.UDPAddr) {
	switch packet.GetY() {
	case "r":
		if bencode.CheckMapPath(packet.Data, "r.nodes") {
			res, err := dht.NewFindNodeResponse(packet)
			if err != nil {
				logx.Debugf("Failed to parse find_node response %s", packet)
			}
			c.onFindNodeResponse(res.Nodes)
			c.metricTrafficCounter.Add(float64(packet.Size()), "in_find_node_response")
		}
	case "q":
		switch packet.GetQ() {
		case "get_peers":
			req := dht.NewGetPeersRequestFromPacket(packet)
			c.onGetPeersRequest(req, addr)
			c.metricTrafficCounter.Add(float64(packet.Size()), "in_get_peers")
		case "announce_peer":
			req := dht.NewAnnouncePeerRequestFromPacket(packet)
			c.onAnnouncePeerRequest(req, addr)
			c.metricTrafficCounter.Add(float64(packet.Size()), "in_announce_peer")
			//default:
			//	logrus.Debugf("Drop illegal query with no query_type")
			//	if logrus.GetLevel() == logrus.DebugLevel {
			//		packet.Print()
			//	}
		}
	case "e":
		errs := packet.Get("e")
		if errs != nil {
			logx.Debugf("DHT error response: %s", errs)
		}
		c.metricDHTReceiveCounter.Inc("error")
		c.metricTrafficCounter.Add(float64(packet.Size()), "in_error")
	default:
		logx.Debugf("Drop illegal packet with no type: %s", packet)
		c.metricTrafficCounter.Add(float64(packet.Size()), "in_unknown")
		return
	}
}

func (c *Crawler) onFindNodeResponse(nodes []*dht.Node) {
	c.metricDHTReceiveCounter.Inc("find_node")
	for _, node := range nodes {
		if c.maxQueueSize > 0 && len(c.neighbours) >= c.maxQueueSize {
			return
		}
		if !node.Addr.IP.IsUnspecified() &&
			!bytes.Equal(c.nodeID, node.NodeID) &&
			node.Addr.Port < 65536 &&
			node.Addr.Port > 0 {
			c.neighbours <- node
		}
	}
}

func (c *Crawler) onGetPeersRequest(req *dht.GetPeersRequest, addr *net.UDPAddr) {
	c.metricDHTReceiveCounter.Inc("get_peers")
	tid := req.GetT()
	if tid == nil {
		logx.Debugf("Drop request with no tid. %s", req)
		return
	}
	nid := req.InfoHash()
	if len(nid) < 20 {
		logx.Debugf("Got get_peer request with illegal infohash %x, drop. %s", nid, req)
		return
	}
	logx.Debugf("Got get_peer request for infohash %x", nid)
	// AlphaReign
	res := dht.NewGetPeersResponse(dht.GetNeighbourID(nid, c.nodeID), req.Token())
	// Official
	//res := dht.NewGetPeersResponse(c.nodeID, req.Token())
	res.SetT(tid)
	_ = c.sendPacket(res.Packet, addr)
	c.metricTrafficCounter.Add(float64(res.Packet.Size()), "out_get_peers_response")
}

func (c *Crawler) onAnnouncePeerRequest(req *dht.AnnouncePeerRequest, addr *net.UDPAddr) {
	c.metricDHTReceiveCounter.Inc("announce_peer")
	tid := req.GetT()
	// AlphaReign
	res := dht.NewEmptyResponsePacket(dht.GetNeighbourID(req.NodeID(), c.nodeID))
	// Official
	// res := dht.NewEmptyResponsePacket(c.nodeID)
	res.SetT(tid)
	_ = c.sendPacket(res, addr)
	c.metricTrafficCounter.Add(float64(res.Size()), "out_announce_peer_response")
	logx.Debugf("Got announce peer %x %s", req.InfoHash(), req.Name())

	var a string
	if req.ImpliedPort() == 0 {
		a = fmt.Sprintf("%s:%d", addr.IP, req.Port())
	} else {
		a = addr.String()
	}
	bt := bittorrent.NewBitTorrent(c.nodeID, req.InfoHash(), a)
	bt.SetTrafficMetricFunc(func(label string, length int) {
		c.metricTrafficCounter.Add(float64(length), label)
	})
	if c.executor.QueueSize() >= c.svcCtx.Config.TorrentMaxQueueSize {
		c.metricCrawlerEvent.Inc("drop_torrent")
		logx.Debugf("Pull torrent task queue full, drop %s", hex.EncodeToString(req.InfoHash()))
	} else {
		c.executor.Commit(bt)
	}
}

func (c *Crawler) pullTorrent(bt *bittorrent.BitTorrent) {
	if c.checkInfoHashExist(bt.InfoHash) {
		return
	}
	c.metricCrawlerEvent.Inc("pull_torrent")
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
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
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
		c.handleTorrent(torrent)
	}
}

// 存在误伤
func (c *Crawler) checkInfoHashExist(infoHash []byte) bool {
	e, err := c.bloomFilter.Exists(infoHash)
	if err != nil {
		logx.Errorf("Failed to read bloom filter, fallback to check %v", err)
		e = false
	}
	if e {
		res, err := c.svcCtx.OceanRpc.IfInfoHashExists(context.TODO(), &oceanclient.IfInfoHashExistsRequest{
			InfoHash: infoHash,
		})
		if err != nil {
			logx.Errorf("Check infohash failed. %v", err)
			return false
		}
		if res.Exists {
			err = c.bloomFilter.Add(infoHash)
			if err != nil {
				logx.Errorf("Failed to update bloom filter. %v", err)
			}
		}
		return res.Exists
	} else {
		return e
	}
}

func (c *Crawler) handleTorrent(torrent *bittorrent.Torrent) {
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
	_, err := c.svcCtx.OceanRpc.CommitTorrent(context.TODO(), req)
	if err != nil {
		logx.Errorf("Failed to commit torrent %s %s. %v", torrent.InfoHash, torrent.Name, err)
		return
	}
	err = c.bloomFilter.Add(req.InfoHash)
	if err != nil {
		logx.Errorf("Failed to add bloom filter. %v", err)
	}
}

func (c *Crawler) LogStats() {
	logx.Infof("Node queue size: %d", len(c.neighbours))
}
