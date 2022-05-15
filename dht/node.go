package dht

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"strconv"
)

const (
	rawNodeLength = 26
)

func GenerateNodeID() []byte {
	hash := sha1.New()
	for i := 0; i < 32; i++ {
		hash.Write([]byte(strconv.Itoa(rand.Int())))
	}
	return hash.Sum(nil)
}

func GetNeighbourID(target, nodeID []byte) []byte {
	return append(target[:10], nodeID[10:]...)
}

type Node struct {
	rawIP   []byte
	rawPort []byte
	Addr    *net.UDPAddr
	NodeID  []byte
}

func NewNodeFromRaw(raw []byte) (*Node, error) {
	node := &Node{}
	node.NodeID = raw[:20]
	node.rawIP = raw[20:24]
	node.rawPort = raw[24:26]
	node.Addr = &net.UDPAddr{
		IP:   node.GetIP(),
		Port: node.GetPort(),
	}
	return node, nil
}

func (n *Node) NodeIDString() string {
	return string(n.NodeID)
}

func (n *Node) GetIP() net.IP {
	return net.IPv4(n.rawIP[0], n.rawIP[1], n.rawIP[2], n.rawIP[3])
}

func (n *Node) GetPort() int {
	return (int)(binary.BigEndian.Uint16(n.rawPort))
}

type PingRequest struct {
	*Packet
}

func NewPingRequest(nodeID []byte) *PingRequest {
	pkt := NewPacket()
	pkt.SetY("q")
	pkt.Set("q", "ping")
	pkt.Set("a", map[string]any{"id": nodeID})
	r := &PingRequest{pkt}
	return r
}

type PingResponse struct {
	*Packet
}

func NewPingResponse(nodeID []byte) *PingResponse {
	pkt := NewPacket()
	pkt.SetY("r")
	pkt.Set("r", map[string]any{"id": nodeID})
	r := &PingResponse{pkt}
	return r
}

func NewPingResponseFromPacket(pkt *Packet) *PingResponse {
	r := &PingResponse{pkt}
	return r
}

func (r *PingResponse) Tid() []byte {
	return r.Packet.GetT()
}

func (r *PingResponse) NodeID() []byte {
	m := r.Packet.Get("r")
	switch m.(type) {
	case map[string]any:
		return m.(map[string]any)["id"].([]byte)
	default:
		return nil
	}
}

type FindNodeRequest struct {
	*Packet
}

func NewFindNodeRequest(nodeID, target []byte) *FindNodeRequest {
	pkt := NewPacket()
	pkt.SetY("q")
	pkt.Set("q", "find_node")
	pkt.Set("a", map[string]any{"id": nodeID, "target": target})
	return &FindNodeRequest{pkt}
}

type FindNodeResponse struct {
	Tid          []byte
	TargetNodeID []byte
	Nodes        []*Node
}

func NewFindNodeResponse(pkt *Packet) (*FindNodeResponse, error) {
	r := &FindNodeResponse{}
	r.Tid = pkt.GetT()
	rMap := pkt.Get("r")
	switch rMap.(type) {
	case map[string]any:
		r.TargetNodeID = rMap.(map[string]any)["id"].([]byte)
		rawNodes := rMap.(map[string]any)["nodes"].([]byte)
		for i := 0; (i+1)*(rawNodeLength) < len(rawNodes); i++ {
			node, err := NewNodeFromRaw(rawNodes[i*rawNodeLength : (i+1)*rawNodeLength])
			if err != nil {
				return nil, err
			}
			r.Nodes = append(r.Nodes, node)
		}
		return r, nil
	default:
		return nil, fmt.Errorf("illegal packet Data structure")
	}
}

func (n *Node) Print() {
	fmt.Printf("ID: %x\nIP: %s\nPort: %d\n\n", n.NodeID, n.GetIP(), n.GetPort())
}
