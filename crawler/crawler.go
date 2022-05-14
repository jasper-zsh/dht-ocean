package crawler

import (
	"bytes"
	"dht-ocean/bencode"
	"dht-ocean/bittorrent"
	"dht-ocean/dht"
	"dht-ocean/dht/protocol"
	"fmt"
	"github.com/sirupsen/logrus"
	"net"
	"time"
)

type Crawler struct {
	addr           *net.UDPAddr
	conn           *net.UDPConn
	nodeID         []byte
	bootstrapNodes []*protocol.Node
	nodes          []*protocol.Node
	ticker         *time.Ticker
	tm             *dht.TransactionManager
	packetBuffers  chan *protocol.Packet
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
		packetBuffers: make(chan *protocol.Packet, 1000),
	}
	return c, nil
}

func (c *Crawler) SetBootstrapNodes(addrs []string) {
	nodes := make([]*protocol.Node, 0, len(addrs))
	for _, strAddr := range addrs {
		addr, err := net.ResolveUDPAddr("udp", strAddr)
		if err != nil {
			logrus.Warnf("Illegal bootstrap node address %s.", strAddr)
			continue
		}
		node := &protocol.Node{
			NodeID: c.nodeID,
			Addr:   addr,
		}
		nodes = append(nodes, node)
	}
	c.bootstrapNodes = nodes
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
	_ = c.conn.Close()
	c.ticker.Stop()
}

func (c *Crawler) listen() {
	buf := make([]byte, 4096)
	for {
		bytes, addr, err := c.conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}
		logrus.Tracef("Read %d bytes from udp %s", bytes, addr)
		msg := make([]byte, bytes)
		copy(msg, buf)
		pkt := protocol.NewPacketFromBuffer(msg)
		pkt.Addr = addr
		c.packetBuffers <- pkt
	}
}

func (c *Crawler) handleMessage() {
	for {
		pkt := <-c.packetBuffers
		err := pkt.Decode()
		if err != nil {
			logrus.Warnf("Failed to parse DHT packet. %v", err)
			continue
		}
		c.onMessage(pkt, pkt.Addr)
	}
}

func (c *Crawler) sendPacket(pkt *protocol.Packet, addr *net.UDPAddr) error {
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
		c.sendFindNode(c.nodeID, protocol.GenerateNodeID(), node.Addr)
		cnt += 1
	}
	logrus.Infof("Sending %d find_node queries.", cnt)
	c.nodes = c.nodes[:0]
}

func (c *Crawler) sendFindNode(nodeID []byte, target []byte, addr *net.UDPAddr) {
	req := protocol.NewFindNodeRequest(nodeID, target)
	req.SetT(c.tm.NextTransactionID())
	_ = c.sendPacket(req.Packet, addr)
}

func (c *Crawler) sendPing(addr *net.UDPAddr) {
	req := protocol.NewPingRequest(c.nodeID)
	req.SetT(c.tm.NextTransactionID())
	_ = c.sendPacket(req.Packet, addr)
}

func (c *Crawler) loop() {
	c.makeNeighbours()
}

func (c *Crawler) onMessage(packet *protocol.Packet, addr *net.UDPAddr) {
	switch packet.GetY() {
	case "r":
		if bencode.CheckMapPath(packet.Data, "r.nodes") {
			res, err := protocol.NewFindNodeResponse(packet)
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
			req := protocol.NewGetPeersRequestFromPacket(packet)
			c.onGetPeersRequest(req, addr)
		case "announce_peer":
			req := protocol.NewAnnouncePeerRequestFromPacket(packet)
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

func (c *Crawler) onFindNodeResponse(nodes []*protocol.Node) {
	for _, node := range nodes {
		if !node.Addr.IP.IsUnspecified() &&
			!bytes.Equal(c.nodeID, node.NodeID) &&
			node.Addr.Port < 65536 &&
			node.Addr.Port > 0 &&
			len(c.nodes) < 2000 {
			c.nodes = append(c.nodes, node)
		}
	}
}

func (c *Crawler) onGetPeersRequest(req *protocol.GetPeersRequest, addr *net.UDPAddr) {
	tid := req.GetT()
	if tid == nil {
		logrus.Debugf("Drop request with no tid.")
		if logrus.GetLevel() >= logrus.DebugLevel {
			req.Print()
		}
		return
	}
	res := protocol.NewGetPeersResponse(protocol.GetNeighbourID(req.InfoHash(), req.NodeID()), req.Token())
	res.SetT(tid)
	_ = c.sendPacket(res.Packet, addr)
}

func (c *Crawler) onAnnouncePeerRequest(req *protocol.AnnouncePeerRequest, addr *net.UDPAddr) {
	tid := req.GetT()
	res := protocol.NewEmptyResponsePacket(protocol.GetNeighbourID(req.NodeID(), c.nodeID))
	res.SetT(tid)
	_ = c.sendPacket(res, addr)
	req.Print()

	go func() {
		var a string
		if req.ImpliedPort() == 0 {
			a = fmt.Sprintf("%s:%d", addr.IP, req.Port())
		} else {
			a = addr.String()
		}
		bt := bittorrent.NewBitTorrent(req.InfoHash(), a)
		err := bt.Start()
		if err != nil {
			logrus.Debugf("Failed to connect to peer to fetch metadata from %s %v", addr, err)
			return
		}
		defer bt.Stop()
		err = bt.GetMetadata()
		if err != nil {
			logrus.Debugf("Failed to fetch metadata from %s %v", addr, err)
		}
	}()
}

func (c *Crawler) LogStats() {
	logrus.Infof("Node queue size: %d", len(c.nodes))
}
