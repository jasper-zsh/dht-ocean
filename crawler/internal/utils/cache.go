package utils

import (
	"context"
	"sync"
	"time"
)

type cacheKey interface{ ~string }

type lrwEntry[V any] struct {
	timeOffset uint16
}

type expireEntry[K cacheKey] struct {
	timeOffset uint16
	key        K
}

type LRWCache[K cacheKey, V any] struct {
	ttl         uint16
	basetime    int64
	maxSize     int
	expireQueue chan expireEntry[K]
	data        map[K]V
	ctx         context.Context
	cancel      context.CancelFunc
	ticker      *time.Ticker
	mu          sync.RWMutex
}

func NewLRWCache[K cacheKey, V any](ctx context.Context, ttl uint16, maxSize int) *LRWCache[K, V] {
	c := &LRWCache[K, V]{
		ttl:         ttl,
		basetime:    time.Now().Unix(),
		maxSize:     maxSize,
		expireQueue: make(chan expireEntry[K], maxSize),
		data:        make(map[K]V, maxSize),
		ticker:      time.NewTicker(time.Second * time.Duration(ttl)),
	}
	c.ctx, c.cancel = context.WithCancel(ctx)
	go c.evict()
	go c.refresh()
	return c
}

func (c *LRWCache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.expireQueue) == c.maxSize {
		e := <-c.expireQueue
		delete(c.data, e.key)
	}
	c.data[key] = value
	c.expireQueue <- expireEntry[K]{
		timeOffset: uint16(time.Now().Unix() - c.basetime),
		key:        key,
	}
}

func (c *LRWCache[K, V]) Get(key K) (value V, exists bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, exists = c.data[key]
	return
}

func (c *LRWCache[K, V]) actualTime(timeOffset uint16) int64 {
	now := time.Now().Unix()
	r := c.basetime + int64(timeOffset)
	if r > now {
		r = c.basetime - int64(c.ttl) + int64(timeOffset)
	}
	return r
}

func (c *LRWCache[K, V]) refresh() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.ticker.C:
			c.basetime = time.Now().Unix()
		}
	}
}

func (c *LRWCache[K, V]) evict() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case e := <-c.expireQueue:
			now := time.Now().Unix()
			ttlTime := c.actualTime(e.timeOffset)
			if ttlTime > now {
				select {
				case <-c.ctx.Done():
					return
				case <-time.After(time.Duration(ttlTime-now) * time.Second):
					c.mu.Lock()
					delete(c.data, e.key)
					c.mu.Unlock()
				}
			} else {
				c.mu.Lock()
				delete(c.data, e.key)
				c.mu.Unlock()
			}
		}
	}
}

func (c *LRWCache[K, V]) Close() {
	c.cancel()
	c.ticker.Stop()
}
