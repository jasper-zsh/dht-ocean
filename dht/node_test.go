package dht

import (
	"fmt"
	"github.com/elliotchance/orderedmap"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDHTConn_FindNode(t *testing.T) {
	nodeId := GenerateNodeID()
	conn, err := NewDHTConn("dht.transmissionbt.com:6881", nodeId)
	if !assert.NoError(t, err) {
		return
	}
	defer conn.Close()
	err = conn.Ping()
	if !assert.NoError(t, err) {
		return
	}
	pong, err := conn.ReadPacket()
	if !assert.NoError(t, err) {
		return
	}
	pong.Print()
	target := pong.Get("r").(*orderedmap.OrderedMap).GetOrDefault("id", []byte{}).([]byte)
	fmt.Printf("got target id %x", target)
	err = conn.FindNode(target)
	if !assert.NoError(t, err) {
		return
	}
	pkt, err := conn.ReadPacket()
	if !assert.NoError(t, err) {
		return
	}
	pkt.Print()
	rawNodes := pkt.Get("r").(*orderedmap.OrderedMap).GetOrDefault("nodes", []byte{}).([]byte)
	assert.Len(t, rawNodes, 8*26)
	res, err := NewFindNodeResponse(pkt)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, target, res.TargetNodeID)
	assert.Len(t, res.Nodes, 8)
	for _, n := range res.Nodes {
		n.Print()
	}
}
