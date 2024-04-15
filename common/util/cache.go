package util

import (
	"context"
	"sync"
	"time"
)

type cacheKey interface{ uint32 | ~string }

type expireEntry[K cacheKey] struct {
	ttl int
	key K
}

type cacheEntry[V any] struct {
	timer *time.Timer
	value V
}

type LRWCache[K cacheKey, V any] struct {
	ttl      int
	maxSize  int
	tokens   chan K
	data     map[K]cacheEntry[V]
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.RWMutex
	blocking bool
}

func NewLRWCache[K cacheKey, V any](ctx context.Context, ttlSeconds int, maxSize int, blocking bool) *LRWCache[K, V] {
	c := &LRWCache[K, V]{
		ttl:      ttlSeconds,
		maxSize:  maxSize,
		tokens:   make(chan K, maxSize),
		data:     make(map[K]cacheEntry[V], maxSize),
		blocking: blocking,
	}
	c.ctx, c.cancel = context.WithCancel(ctx)
	return c
}

func (c *LRWCache[K, V]) Set(key K, value V) {
	if !c.blocking {
		if len(c.tokens) == c.maxSize {
			c.Delete(<-c.tokens)
		}
	}
	c.tokens <- key
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = cacheEntry[V]{
		timer: time.AfterFunc(time.Duration(c.ttl)*time.Second, func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.delete(key)
		}),
		value: value,
	}
}

func (c *LRWCache[K, V]) Get(key K) (value V, exists bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, exists = c.get(key)
	return
}

func (c *LRWCache[K, V]) get(key K) (value V, exists bool) {
	entry, exists := c.data[key]
	if exists {
		value = entry.value
	}
	return
}

func (c *LRWCache[K, V]) GetAndRemove(key K) (value V, exists bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	value, exists = c.get(key)
	c.delete(key)
	return
}

func (c *LRWCache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.delete(key)
}

func (c *LRWCache[K, V]) delete(key K) {
	entry, ok := c.data[key]
	if !ok {
		return
	}
	entry.timer.Stop()
	delete(c.data, key)
	<-c.tokens
}
