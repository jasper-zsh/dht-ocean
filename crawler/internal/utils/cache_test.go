package utils

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLRWCache(t *testing.T) {
	c := NewLRWCache[string, struct{}](context.Background(), 5, 100)
	c.Set("foo", struct{}{})
	_, ok := c.Get("foo")
	assert.True(t, ok)
	time.Sleep(5001 * time.Millisecond)
	_, ok = c.Get("foo")
	assert.False(t, ok)
	c.Set("foo", struct{}{})
	_, ok = c.Get("foo")
	assert.True(t, ok)
	time.Sleep(5001 * time.Millisecond)
	_, ok = c.Get("foo")
	assert.False(t, ok)
}
