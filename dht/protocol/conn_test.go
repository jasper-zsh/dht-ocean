package protocol

import (
	"dht-ocean/dht"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDHTConn_Ping(t *testing.T) {
	nodeId := dht.GenerateNodeID()
	conn, err := NewDHTConn("dht.transmissionbt.com:6881", nodeId)
	if !assert.NoError(t, err) {
		return
	}
	defer conn.Close()
	err = conn.Ping()
	if !assert.NoError(t, err) {
		return
	}
	buf := make([]byte, 4096)
	cnt, err := conn.conn.Read(buf)
	if !assert.NoError(t, err) {
		return
	}
	fmt.Printf("Received %d bytes response: %s\n", cnt, buf)
	pkt, err := dht.NewPacketFromBuffer(buf)
	if assert.NoError(t, err) {
		pkt.Print()
	}
}
