package svc

import (
	"bytes"
	"context"
	"dht-ocean/common/bencode"
	"dht-ocean/common/bittorrent"
	"dht-ocean/common/dht"
	"dht-ocean/ocean/ocean"
	"dht-ocean/ocean/oceanclient"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"github.com/zeromicro/go-zero/core/bloom"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/threading"
	"net"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"
)

type InfoHashFilter func(infoHash []byte) bool

type Crawler struct {
	addr           *net.UDPAddr
	conn           *net.UDPConn
	nodeID         []byte
	bootstrapNodes []*dht.Node
	nodes          []*dht.Node
	ticker         *time.Ticker
	tm             *dht.TransactionManager
	packetBuffers  chan *dht.Packet
	maxQueueSize   int
	svcCtx         *ServiceContext
	bloomFilter    *bloom.Filter
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
	nodeID, err := os.ReadFile("node_id")
	if err != nil {
		logx.Infof("Cannot read nodeID, generated randomly.")
		nodeID = dht.GenerateNodeID()
		err = os.WriteFile("node_id", nodeID, 0666)
		if err != nil {
			logx.Errorf("Failed to write nodeID")
		}
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
	}
	redis := c.svcCtx.Config.Redis.NewRedis()
	c.bloomFilter = bloom.New(redis, "torrent_bloom", 1024*1024*5)
	c.SetBootstrapNodes(svcCtx.Config.BootstrapNodes)
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
			addr, err := net.ResolveUDPAddr("udp", strAddr)
			if err != nil {
				logx.Errorf("Illegal bootstrap node address %s.", strAddr)
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
	routineGroup.RunSafe(func() {
		for {
			select {
			case <-c.ticker.C:
				c.loop()
			}
		}
	})
	routineGroup.Wait()
}

func (c *Crawler) Stop() {
	if c.conn != nil {
		_ = c.conn.Close()
	}
	if c.ticker != nil {
		c.ticker.Stop()
	}
}

func (c *Crawler) listen() {
	buf := make([]byte, 65536)
	for {
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

func (c *Crawler) handleMessage() {
	for {
		pkt := <-c.packetBuffers
		err := pkt.Decode()
		if err != nil {
			logx.Debugf("Failed to parse DHT packet. %s %v", pkt, err)
			continue
		}
		c.onMessage(pkt, pkt.Addr)
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
	cnt := 0
	c.nodes = append(c.nodes, c.bootstrapNodes...)
	for _, node := range c.nodes {
		// AlphaReign
		c.sendFindNode(dht.GetNeighbourID(node.NodeID, c.nodeID), dht.GenerateNodeID(), node.Addr)
		// Official
		// c.sendFindNode(c.nodeID, dht.GenerateNodeID(), node.Addr)
		cnt += 1
	}
	logx.Debugf("Sending %d find_node queries.", cnt)
	c.nodes = c.nodes[:0]
}

func (c *Crawler) sendFindNode(nodeID []byte, target []byte, addr *net.UDPAddr) {
	//req := dht.NewFindNodeRequest(dht.GetNeighbourID(nodeID, c.nodeID), target)
	req := dht.NewFindNodeRequest(nodeID, target)
	req.SetT(c.tm.NextTransactionID())
	_ = c.sendPacket(req.Packet, addr)
}

func (c *Crawler) sendPing(addr *net.UDPAddr) {
	req := dht.NewPingRequest(c.nodeID)
	req.SetT(c.tm.NextTransactionID())
	_ = c.sendPacket(req.Packet, addr)
}

func (c *Crawler) loop() {
	c.LogStats()
	c.makeNeighbours()
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
		}
	case "q":
		switch packet.GetQ() {
		case "get_peers":
			req := dht.NewGetPeersRequestFromPacket(packet)
			c.onGetPeersRequest(req, addr)
		case "announce_peer":
			req := dht.NewAnnouncePeerRequestFromPacket(packet)
			c.onAnnouncePeerRequest(req, addr)
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
	default:
		logx.Debugf("Drop illegal packet with no type: %s", packet)
		return
	}
}

func (c *Crawler) onFindNodeResponse(nodes []*dht.Node) {
	for _, node := range nodes {
		if c.maxQueueSize > 0 && len(c.nodes) > c.maxQueueSize {
			return
		}
		if !node.Addr.IP.IsUnspecified() &&
			!bytes.Equal(c.nodeID, node.NodeID) &&
			node.Addr.Port < 65536 &&
			node.Addr.Port > 0 {
			c.nodes = append(c.nodes, node)
		}
	}
}

func (c *Crawler) onGetPeersRequest(req *dht.GetPeersRequest, addr *net.UDPAddr) {
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
}

func (c *Crawler) onAnnouncePeerRequest(req *dht.AnnouncePeerRequest, addr *net.UDPAddr) {
	tid := req.GetT()
	// AlphaReign
	res := dht.NewEmptyResponsePacket(dht.GetNeighbourID(req.NodeID(), c.nodeID))
	// Official
	// res := dht.NewEmptyResponsePacket(c.nodeID)
	res.SetT(tid)
	_ = c.sendPacket(res, addr)
	logx.Debugf("Got announce peer %x %s", req.InfoHash(), req.Name())

	go func() {
		if c.checkInfoHashExist(req.InfoHash()) {
			return
		}
		var a string
		if req.ImpliedPort() == 0 {
			a = fmt.Sprintf("%s:%d", addr.IP, req.Port())
		} else {
			a = addr.String()
		}
		bt := bittorrent.NewBitTorrent(c.nodeID, req.InfoHash(), a)
		err := bt.Start()
		if err != nil {
			logx.Debugf("Failed to connect to peer to fetch metadata from %s %v", addr, err)
			return
		}
		defer bt.Stop()
		metadata, err := bt.GetMetadata()
		if err != nil {
			logx.Debugf("Failed to fetch metadata from %s %+v", addr, err)
			return
		}
		torrent := &bittorrent.Torrent{
			InfoHash: req.InfoHash(),
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
			go c.handleTorrent(torrent)
		}
	}()
}

// 存在误伤
func (c *Crawler) checkInfoHashExist(infoHash []byte) bool {
	e, err := c.bloomFilter.Exists(infoHash)
	if err != nil {
		logx.Errorf("Failed to read bloom filter, fallback to check %v", err)
		e = false
	}
	if !e {
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
	}
	return true
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
}

func (c *Crawler) LogStats() {
	logx.Infof("Node queue size: %d", len(c.nodes))
}
