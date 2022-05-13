package protocol

import (
	"dht-ocean/dht"
	"encoding/binary"
	"fmt"
	"github.com/elliotchance/orderedmap"
)

const (
	rawNodeLength = 26
)

type Node struct {
	rawIP   []byte
	rawPort []byte
	Addr    string
	Port    int
	NodeID  []byte
	conn    *DHTConn
}

func NewNodeFromRaw(raw []byte) (*Node, error) {
	node := &Node{}
	node.NodeID = raw[:20]
	node.rawIP = raw[20:24]
	node.rawPort = raw[24:26]
	node.Addr = node.GetIP()
	node.Port = node.GetPort()
	return node, nil
}

func (n *Node) GetIP() string {
	return fmt.Sprintf("%d.%d.%d.%d", n.rawIP[0], n.rawIP[1], n.rawIP[2], n.rawIP[3])
}

func (n *Node) GetPort() int {
	return (int)(binary.BigEndian.Uint16(n.rawPort))
}

func (n *Node) Connect() error {
	conn, err := NewDHTConn(fmt.Sprintf("%s:%d", n.Addr, n.Port), n.NodeID)
	if err != nil {
		return err
	}
	n.conn = conn
	return nil
}

func (n *Node) Disconnect() error {
	return n.conn.Close()
}

func (n *Node) FindNode(target []byte) (*FindNodeResponse, error) {
	if target == nil {
		target = dht.GenerateNodeID()
	}
	err := n.conn.FindNode(target)
	if err != nil {
		return nil, err
	}

	pkt, err := n.conn.ReadPacket()
	if err != nil {
		return nil, err
	}
	pkt.Print()

	r, err := NewFindNodeResponse(pkt)
	if err != nil {
		return nil, err
	}
	return r, nil
}

type PingResponse struct {
	Tid    []byte
	NodeID []byte
}

func NewPingResponse(pkt *dht.Packet) (*PingResponse, error) {
	r := &PingResponse{}
	r.Tid = pkt.GetT()
	m := pkt.Get("r")
	switch m.(type) {
	case *orderedmap.OrderedMap:
		r.NodeID = m.(*orderedmap.OrderedMap).GetOrDefault("id", []byte{}).([]byte)
		return r, nil
	default:
		return nil, fmt.Errorf("illegal ping response")
	}
}

type FindNodeResponse struct {
	Tid          []byte
	TargetNodeID []byte
	Nodes        []*Node
}

func NewFindNodeResponse(pkt *dht.Packet) (*FindNodeResponse, error) {
	r := &FindNodeResponse{}
	r.Tid = pkt.GetT()
	rMap := pkt.Get("r")
	switch rMap.(type) {
	case *orderedmap.OrderedMap:
		r.TargetNodeID = rMap.(*orderedmap.OrderedMap).GetOrDefault("id", []byte{}).([]byte)
		rawNodes := rMap.(*orderedmap.OrderedMap).GetOrDefault("nodes", []byte{}).([]byte)
		for i := 0; i*rawNodeLength < len(rawNodes); i++ {
			node, err := NewNodeFromRaw(rawNodes[i : i+rawNodeLength])
			if err != nil {
				return nil, err
			}
			r.Nodes = append(r.Nodes, node)
		}
		return r, nil
	default:
		return nil, fmt.Errorf("illegal packet data structure")
	}
}

func (n *Node) Print() {
	fmt.Printf("ID: %x\nIP: %s\nPort: %d\n\n", n.NodeID, n.GetIP(), n.GetPort())
}
