package bencode

import (
	"github.com/elliotchance/orderedmap"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBDecode_decodeString(t *testing.T) {
	pkt := "4:spam"
	str, offset, err := decodeAny([]byte(pkt), 0)
	if assert.NoError(t, err) {
		assert.Equal(t, len(pkt), offset)
		assert.Equal(t, []byte("spam"), str)
	}
}

func TestBDecode_decodeInt(t *testing.T) {
	pkt := "i123432e"
	i, offset, err := decodeAny([]byte(pkt), 0)
	if assert.NoError(t, err) {
		assert.Equal(t, len(pkt), offset)
		assert.Equal(t, 123432, i)
	}
}

func TestBDecode_decodeList(t *testing.T) {
	pkt := "li123e2:aae"
	list, offset, err := decodeAny([]byte(pkt), 0)
	if assert.NoError(t, err) {
		assert.Equal(t, len(pkt), offset)
		assert.IsType(t, []interface{}{}, list)
		assert.Equal(t, 123, list.([]interface{})[0])
		assert.Equal(t, []byte("aa"), list.([]interface{})[1])
	}
}

func TestBDecode_decodeMap(t *testing.T) {
	pkt := "d3:foo3:bar6:foobar3:baze"
	m, offset, err := decodeAny([]byte(pkt), 0)
	if assert.NoError(t, err) {
		assert.Equal(t, len(pkt), offset)
		assert.IsType(t, &orderedmap.OrderedMap{}, m)
		v, _ := m.(*orderedmap.OrderedMap).Get("foo")
		assert.Equal(t, v, []byte("bar"))
		v, _ = m.(*orderedmap.OrderedMap).Get("foobar")
		assert.Equal(t, v, []byte("baz"))
	}
}
