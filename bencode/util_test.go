package bencode

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheckMapPath(t *testing.T) {
	m := map[string]any{
		"foo": "bar",
		"bar": map[string]any{
			"baz": "foobar",
		},
	}
	assert.True(t, CheckMapPath(m, "foo"))
	assert.False(t, CheckMapPath(m, "baz"))
	assert.True(t, CheckMapPath(m, "bar"))
	assert.True(t, CheckMapPath(m, "bar.baz"))
	assert.False(t, CheckMapPath(m, "bar.foo"))

}
