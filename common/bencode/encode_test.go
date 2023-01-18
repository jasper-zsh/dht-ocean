package bencode

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func Test_encodeDict(t *testing.T) {
	b := &strings.Builder{}
	err := encodeMap(b, map[string]interface{}{
		"a": map[string]any{
			"id": "abcdefghij0123456789",
		},
		"q": "ping",
		"t": "aa",
		"y": "q",
	})
	if assert.NoError(t, err) {
		assert.Equal(t, "d1:ad2:id20:abcdefghij0123456789e1:q4:ping1:t2:aa1:y1:qe", b.String())
	}
}
