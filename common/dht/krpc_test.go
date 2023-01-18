package dht

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPacket_Encode(t *testing.T) {
	pkt := NewPacket()
	pkt.SetError(201, "A Generic Error Ocurred")
	pkt.SetT([]byte("aa"))
	pkt.SetY("e")
	result, err := pkt.Encode()
	if assert.NoError(t, err) {
		assert.Equal(t, "d1:eli201e23:A Generic Error Ocurrede1:t2:aa1:y1:ee", result)
	}
}
