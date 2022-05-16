package crawler

import (
	"bytes"
	"dht-ocean/bencode"
	"dht-ocean/bittorrent"
	"dht-ocean/dht"
	"fmt"
	"github.com/sirupsen/logrus"
	"net"
	"time"
)

type InfoHashFilter func(infoHash []byte) bool

type Crawler struct {
	addr            *net.UDPAddr
	conn            *net.UDPConn
	nodeID          []byte
	bootstrapNodes  []*dht.Node
	nodes           []*dht.Node
	ticker          *time.Ticker
	tm              *dht.TransactionManager
	packetBuffers   chan *dht.Packet
	torrentHandlers []bittorrent.TorrentHandler
	infoHashFilter  InfoHashFilter
	maxQueueSize    int
}

func NewCrawler(address string, nodeID []byte) (*Crawler, error) {
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		logrus.Errorf("Failed to resolve listen address %s. %v", address, err)
		return nil, err
	}

	c := &Crawler{
		addr:          addr,
		nodeID:        nodeID,
		tm:            &dht.TransactionManager{},
		packetBuffers: make(chan *dht.Packet, 1000),
	}
	return c, nil
}

func (c *Crawler) SetMaxQueueSize(size int) {
	c.maxQueueSize = size
}

func (c *Crawler) SetBootstrapNodes(addrs []string) {
	nodes := make([]*dht.Node, 0, len(addrs))
	for _, strAddr := range addrs {
		addr, err := net.ResolveUDPAddr("udp", strAddr)
		if err != nil {
			logrus.Warnf("Illegal bootstrap node address %s.", strAddr)
			continue
		}
		node := &dht.Node{
			NodeID: c.nodeID,
			Addr:   addr,
		}
		nodes = append(nodes, node)
	}
	c.bootstrapNodes = nodes
}

func (bt *Crawler) AddTorrentHandler(handler bittorrent.TorrentHandler) {
	bt.torrentHandlers = append(bt.torrentHandlers, handler)
}

func (c *Crawler) SetInfoHashFilter(filter InfoHashFilter) {
	c.infoHashFilter = filter
}

func (c *Crawler) Run() error {
	conn, err := net.ListenUDP("udp", c.addr)
	if err != nil {
		logrus.Errorf("Failed to listen udp on %s. %v", c.addr, err)
		return err
	}
	c.conn = conn

	c.ticker = time.NewTicker(time.Second)
	go func() {
		c.listen()
	}()
	go func() {
		c.handleMessage()
	}()
	go func() {
		for {
			select {
			case <-c.ticker.C:
				c.loop()
			}
		}
	}()
	return nil
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
		logrus.Tracef("Read %d bytes from udp %s", transfered, addr)
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
			logrus.Warnf("Failed to parse DHT packet. %s %v", pkt, err)
			continue
		}
		c.onMessage(pkt, pkt.Addr)
	}
}

func (c *Crawler) sendPacket(pkt *dht.Packet, addr *net.UDPAddr) error {
	encoded, err := pkt.Encode()
	if err != nil {
		logrus.Errorf("Failed to encode packet. %v", err)
		pkt.Print()
		return err
	}
	bytes, err := c.conn.WriteToUDP([]byte(encoded), addr)
	if err != nil {
		logrus.Errorf("Failed to write to udp %s %v", addr, err)
		return err
	}
	logrus.Tracef("Send %d bytes to %s", bytes, addr)
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
	logrus.Debugf("Sending %d find_node queries.", cnt)
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
	c.makeNeighbours()
}

func (c *Crawler) onMessage(packet *dht.Packet, addr *net.UDPAddr) {
	switch packet.GetY() {
	case "r":
		if bencode.CheckMapPath(packet.Data, "r.nodes") {
			res, err := dht.NewFindNodeResponse(packet)
			if err != nil {
				logrus.Debugf("Failed to parse find_node response")
				if logrus.GetLevel() >= logrus.DebugLevel {
					packet.Print()
				}
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
			logrus.Debugf("DHT error response: %s", errs)
		}
	default:
		logrus.Debugf("Drop illegal packet with no type")
		if logrus.GetLevel() >= logrus.DebugLevel {
			packet.Print()
		}
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
		logrus.Debugf("Drop request with no tid.")
		if logrus.GetLevel() >= logrus.DebugLevel {
			req.Print()
		}
		return
	}
	// AlphaReign
	res := dht.NewGetPeersResponse(dht.GetNeighbourID(req.InfoHash(), c.nodeID), req.Token())
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
	logrus.Debugf("Got announce peer %x %s", req.InfoHash(), req.Name())

	go func() {
		if c.infoHashFilter != nil && !c.infoHashFilter(req.InfoHash()) {
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
			logrus.Debugf("Failed to connect to peer to fetch metadata from %s %v", addr, err)
			return
		}
		defer bt.Stop()
		torrent, err := bt.GetTorrent()
		if err != nil {
			logrus.Debugf("Failed to fetch metadata from %s %+v", addr, err)
			return
		}
		logrus.Debugf("Got torrent %s with %d files", torrent.Name(), len(torrent.Files()))
		if torrent.HasFiles() {
			for _, handler := range c.torrentHandlers {
				go handler.HandleTorrent(torrent)
			}
		}
	}()
}

func (c *Crawler) LogStats() {
	logrus.Infof("Node queue size: %d", len(c.nodes))
}
