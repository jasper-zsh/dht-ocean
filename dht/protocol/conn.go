package protocol

import (
	"dht-ocean/dht"
	"net"
	"unsafe"
)

type DHTResponseHandler func(pkt *dht.Packet)

type DHTConn struct {
	conn              *net.UDPConn
	nodeId            []byte
	nextTransactionID uint16
	responseHandlers  map[string][]DHTResponseHandler
}

func NewDHTConn(addr string, nodeId []byte) (*DHTConn, error) {
	ret := &DHTConn{
		nodeId:           nodeId,
		responseHandlers: make(map[string][]DHTResponseHandler),
	}
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, err
	}
	ret.conn = conn

	return ret, nil
}

func (c *DHTConn) Close() error {
	err := c.conn.Close()
	if err != nil {
		return err
	}
	return nil
}

func (c *DHTConn) nextTransaction() []byte {
	tid := *(*[2]byte)(unsafe.Pointer(&c.nextTransactionID))
	c.nextTransactionID += 1
	return []byte{tid[0], tid[1]}
}

func (c *DHTConn) RegisterResponseHandler(queryType string, h DHTResponseHandler) {
	c.responseHandlers[queryType] = append(c.responseHandlers[queryType], h)
}

func (c *DHTConn) ReadPacket() (*dht.Packet, error) {
	buf := make([]byte, 4096)
	_, _, err := c.conn.ReadFromUDP(buf)
	if err != nil {
		return nil, err
	}
	pkt, err := dht.NewPacketFromBuffer(buf)
	if err != nil {
		return nil, err
	}
	return pkt, nil
}

func (c *DHTConn) Ping() error {
	pkt := dht.NewPacket()
	pkt.SetY("q")
	pkt.SetKey("q", "ping")
	pkt.SetKey("a", map[string]interface{}{"id": c.nodeId})
	pkt.SetT(c.nextTransaction())
	encoded, err := pkt.Encode()
	if err != nil {
		return err
	}
	_, err = c.conn.Write([]byte(encoded))
	if err != nil {
		return err
	}
	return nil
}

func (c *DHTConn) FindNode(target []byte) error {
	pkt := dht.NewPacket()
	pkt.SetT(c.nextTransaction())
	pkt.SetY("q")
	pkt.SetKey("q", "find_node")
	pkt.SetKey("a", map[string]interface{}{
		"id":     c.nodeId,
		"target": target,
	})
	encoded, err := pkt.Encode()
	if err != nil {
		return err
	}
	_, err = c.conn.Write([]byte(encoded))
	if err != nil {
		return err
	}
	return nil
}
