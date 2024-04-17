package svc

import (
	"bytes"
	"context"
	"dht-ocean/common/bencode"
	"dht-ocean/common/dht"
	"dht-ocean/common/util"
	"dht-ocean/proxy"
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/juju/errors"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/metric"
	"github.com/zeromicro/go-zero/core/threading"
	"golang.org/x/time/rate"
)

const (
	metricsNamespace = "dht_ocean"
	metricsSubsystem = "crawler"
)

var (
	metricDHTSendCounter    metric.CounterVec
	metricDHTReceiveCounter metric.CounterVec
	metricTrafficCounter    metric.CounterVec
	metricCrawlerEvent      metric.CounterVec
	metricQueueSize         metric.GaugeVec
)

func init() {
	metricDHTSendCounter = metric.NewCounterVec(&metric.CounterVecOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "dht_send",
		Labels:    []string{"type"},
	})
	metricDHTReceiveCounter = metric.NewCounterVec(&metric.CounterVecOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "dht_receive",
		Labels:    []string{"type"},
	})
	metricTrafficCounter = metric.NewCounterVec(&metric.CounterVecOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "traffic",
		Labels:    []string{"type"},
	})
	metricQueueSize = metric.NewGaugeVec(&metric.GaugeVecOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "queue_size",
		Labels:    []string{"type"},
	})
	metricCrawlerEvent = metric.NewCounterVec(&metric.CounterVecOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystem,
		Name:      "crawler_event",
		Labels:    []string{"event"},
	})
}

type InfoHashFilter func(infoHash []byte) bool

type outPacket struct {
	data []byte
	addr net.Addr
}

type Crawler struct {
	ctx             context.Context
	cancel          context.CancelFunc
	socks5Proxy     string
	proxy           string
	addr            *net.UDPAddr
	conn            net.PacketConn
	nodeID          []byte
	bootstrapNodes  []*dht.Node
	neighbours      chan *dht.Node
	seenNodes       *util.LRWCache[string, struct{}]
	findNodeLimiter *rate.Limiter
	ticker          *time.Ticker
	tm              *dht.TransactionManager
	rawPackets      chan *dht.Packet
	decodedPackets  chan *dht.Packet
	outPackets      chan *outPacket
	maxQueueSize    int
	svcCtx          *ServiceContext

	connLock sync.RWMutex
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
		socks5Proxy:    svcCtx.Config.Socks5Proxy,
		proxy:          svcCtx.Config.Proxy,
		addr:           addr,
		nodeID:         nodeID,
		tm:             &dht.TransactionManager{},
		rawPackets:     make(chan *dht.Packet, 1000),
		decodedPackets: make(chan *dht.Packet, 1000),
		outPackets:     make(chan *outPacket, 1000),
		maxQueueSize:   svcCtx.Config.MaxQueueSize,
		svcCtx:         svcCtx,
		neighbours:     make(chan *dht.Node, svcCtx.Config.MaxQueueSize),
	}
	c.SetBootstrapNodes(svcCtx.Config.BootstrapNodes)
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.seenNodes = util.NewLRWCache[string, struct{}](c.ctx, svcCtx.Config.SeenNodeTTL, svcCtx.Config.MaxSeenNodeSize, false)

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
	counter := 0
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.ticker.C:
			metricQueueSize.Set(float64(len(c.neighbours)), "neighbours")
			metricQueueSize.Set(float64(len(c.rawPackets)), "incoming_packets")
			metricQueueSize.Set(float64(len(c.decodedPackets)), "decoded_packets")
			metricQueueSize.Set(float64(len(c.outPackets)), "outgoing_packets")
			if len(c.neighbours) == 0 {
				counter += 1
				if counter%30 == 0 {
					logx.Infof("Running out of neighbours, sending %d bootstrap nodes", len(c.bootstrapNodes))
					counter = 0
				}
				for _, node := range c.bootstrapNodes {
					c.neighbours <- node
				}
			}
		}
	}
}

func (c *Crawler) Start() {
	err := c.connect()
	if err != nil {
		panic(err)
	}

	c.ticker = time.NewTicker(time.Second)
	routineGroup := threading.NewRoutineGroup()
	routineGroup.Run(c.bootstrap)
	routineGroup.Run(c.sendLoop)
	routineGroup.Run(c.neighbourLoop)
	for i := 0; i < c.svcCtx.Config.DecoderThreads; i++ {
		routineGroup.Run(c.decodeLoop)
	}
	for i := 0; i < c.svcCtx.Config.HandlerThreads; i++ {
		routineGroup.Run(c.handlerLoop)
	}
	routineGroup.Wait()
}

func (c *Crawler) connect() error {
	c.connLock.Lock()
	defer c.connLock.Unlock()
	err := c._connect()
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (c *Crawler) disconnect() {
	c.connLock.Lock()
	defer c.connLock.Unlock()
	c._disconnect()
}

func (c *Crawler) reconnect() error {
	c.connLock.Lock()
	defer c.connLock.Unlock()
	c._disconnect()
	err := c._connect()
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (c *Crawler) _connect() error {
	if c.conn != nil {
		return nil
	}
	var err error
	if len(c.proxy) > 0 {
		proxy := proxy.NewProxyClient(proxy.ProxyClientOptions{
			Server:  c.proxy,
			BufSize: c.svcCtx.Config.ProxyBufSize,
		})
		c.conn, err = proxy.ListenUDP(c.addr.AddrPort().Port())
		if err != nil {
			logx.Errorf("Failed to listen udp via proxy. %+v", err)
			return errors.Trace(err)
		}
	} else {
		c.conn, err = net.ListenUDP("udp", c.addr)
		if err != nil {
			logx.Errorf("Failed to listen udp on %s. %v", c.addr, err)
			return errors.Trace(err)
		}
	}
	go c.listen(c.conn)
	return nil
}

func (c *Crawler) _disconnect() {
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

func (c *Crawler) Stop() {
	c.disconnect()
	if c.ticker != nil {
		c.ticker.Stop()
	}
	c.cancel()
}

func (c *Crawler) listen(conn net.PacketConn) {
	logx.Info("Connection established, start listening")
	buf := make([]byte, 65536)
	for {
		transfered, addr, err := conn.ReadFrom(buf)
		if err != nil {
			logx.Errorf("Connection broken: %+v", err)
			c.disconnect()
			return
		}
		logx.Debugf("Read %d bytes from udp %s", transfered, addr)
		msg := make([]byte, transfered)
		copy(msg, buf)
		pkt := dht.NewPacketFromBuffer(msg)
		pkt.Addr = addr
		c.rawPackets <- pkt
	}
}

func (c *Crawler) decodeLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case pkt := <-c.rawPackets:
			err := pkt.Decode()
			if err != nil {
				logx.Debugf("Failed to parse DHT packet. %s %v", pkt, err)
				continue
			}
			c.decodedPackets <- pkt
		}
	}
}

func (c *Crawler) handlerLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case pkt := <-c.decodedPackets:
			c.onMessage(pkt, pkt.Addr)
		}
	}
}

func (c *Crawler) sendLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case pkt := <-c.outPackets:
			addrPort, err := netip.ParseAddrPort(pkt.addr.String())
			if err != nil {
				logx.Errorf("[sendLoop] Failed to parse addr: %+v", err)
				continue
			}
			if addrPort.Port() == 0 {
				logx.Errorf("[sendLoop] Illegal addr: %s", pkt.addr.String())
				continue
			}
			if !addrPort.IsValid() {
				logx.Errorf("[sendLoop] Illegal addr: %s", pkt.addr.String())
				continue
			}
			if c.conn == nil {
				err = c.connect()
				if err != nil {
					logx.Errorf("[sendLoop] connect failed: %+v", err)
					continue
				}
			}
			bytes, err := c.conn.WriteTo(pkt.data, pkt.addr)
			if err != nil {
				logx.Errorf("Failed to write to udp %s %v", pkt.addr, err)
				c.disconnect()
				continue
			}
			logx.Debugf("Send %d bytes to %s", bytes, pkt.addr)
		}
	}
}

func (c *Crawler) neighbourLoop() {
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

func (c *Crawler) sendPacket(pkt *dht.Packet, addr net.Addr) error {
	pkt.Addr = addr
	encoded, err := pkt.Encode()
	if err != nil {
		logx.Errorf("Failed to encode packet: %s, %v", pkt, err)
		return err
	}
	c.outPackets <- &outPacket{
		data: []byte(encoded),
		addr: addr,
	}
	return nil
}

func (c *Crawler) sendFindNode(nodeID []byte, target []byte, addr *net.UDPAddr) {
	//req := dht.NewFindNodeRequest(dht.GetNeighbourID(nodeID, c.nodeID), target)
	req := dht.NewFindNodeRequest(nodeID, target)
	req.SetT(c.tm.NextTransactionID())
	_ = c.sendPacket(req.Packet, addr)
	metricDHTSendCounter.Inc("find_node")
	metricTrafficCounter.Add(float64(req.Packet.Size()), "out_find_node")
}

func (c *Crawler) sendPing(addr *net.UDPAddr) {
	req := dht.NewPingRequest(c.nodeID)
	req.SetT(c.tm.NextTransactionID())
	_ = c.sendPacket(req.Packet, addr)
	metricDHTSendCounter.Inc("ping")
	metricTrafficCounter.Add(float64(req.Packet.Size()), "out_ping")
}

func (c *Crawler) onMessage(packet *dht.Packet, addr net.Addr) {
	switch packet.GetY() {
	case "r":
		if bencode.CheckMapPath(packet.Data, "r.nodes") {
			res, err := dht.NewFindNodeResponse(packet)
			if err != nil {
				logx.Debugf("Failed to parse find_node response %s", packet)
			}
			c.onFindNodeResponse(res.Nodes)
			metricTrafficCounter.Add(float64(packet.Size()), "in_find_node_response")
		}
	case "q":
		switch packet.GetQ() {
		case "get_peers":
			req := dht.NewGetPeersRequestFromPacket(packet)
			c.onGetPeersRequest(req, addr)
			metricTrafficCounter.Add(float64(packet.Size()), "in_get_peers")
		case "announce_peer":
			req := dht.NewAnnouncePeerRequestFromPacket(packet)
			c.onAnnouncePeerRequest(req, addr)
			metricTrafficCounter.Add(float64(packet.Size()), "in_announce_peer")
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
		metricDHTReceiveCounter.Inc("error")
		metricTrafficCounter.Add(float64(packet.Size()), "in_error")
	default:
		logx.Debugf("Drop illegal packet with no type: %s", packet)
		metricTrafficCounter.Add(float64(packet.Size()), "in_unknown")
		return
	}
}

func (c *Crawler) onFindNodeResponse(nodes []*dht.Node) {
	metricDHTReceiveCounter.Inc("find_node")
	for _, node := range nodes {
		if c.maxQueueSize > 0 && len(c.neighbours) >= c.maxQueueSize {
			metricCrawlerEvent.Inc("drop_node")
			return
		}
		if !node.Addr.IP.IsUnspecified() &&
			!bytes.Equal(c.nodeID, node.NodeID) &&
			node.Addr.Port < 65536 &&
			node.Addr.Port > 0 {
			key := node.Addr.String()
			_, seen := c.seenNodes.Get(key)
			if !seen {
				c.seenNodes.Set(key, struct{}{})
				c.neighbours <- node
				metricCrawlerEvent.Inc("queue_node")
			} else {
				metricCrawlerEvent.Inc("seen_node")
			}
		}
	}
}

func (c *Crawler) onGetPeersRequest(req *dht.GetPeersRequest, addr net.Addr) {
	metricDHTReceiveCounter.Inc("get_peers")
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
	metricDHTSendCounter.Inc("get_peers_response")
	metricTrafficCounter.Add(float64(res.Packet.Size()), "out_get_peers_response")
}

func (c *Crawler) onAnnouncePeerRequest(req *dht.AnnouncePeerRequest, addr net.Addr) {
	metricDHTReceiveCounter.Inc("announce_peer")
	tid := req.GetT()
	// AlphaReign
	res := dht.NewEmptyResponsePacket(dht.GetNeighbourID(req.NodeID(), c.nodeID))
	// Official
	// res := dht.NewEmptyResponsePacket(c.nodeID)
	res.SetT(tid)
	_ = c.sendPacket(res, addr)
	metricDHTSendCounter.Inc("announce_peer_response")
	metricTrafficCounter.Add(float64(res.Size()), "out_announce_peer_response")
	logx.Debugf("Got announce peer %x %s", req.InfoHash(), req.Name())

	var a string
	if req.ImpliedPort() == 0 {
		addrPort, _ := netip.ParseAddrPort(addr.String())
		a = fmt.Sprintf("%s:%d", addrPort.Addr().String(), req.Port())
	} else {
		a = addr.String()
	}

	c.svcCtx.TorrentFetcher.Push(TorrentRequest{
		NodeID:   c.nodeID,
		InfoHash: req.InfoHash(),
		Addr:     a,
	})
}

func (c *Crawler) LogStats() {
	logx.Infof("Node queue size: %d", len(c.neighbours))
}
