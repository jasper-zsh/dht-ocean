package util

import (
	"encoding/binary"
	"io"
	"sync"

	"github.com/spaolacci/murmur3"
)

const (
	bitsPerByte = 8
)

type BloomFilter struct {
	lock sync.RWMutex `json:"-"`

	m    uint64
	n    uint64
	k    uint8
	keys []byte
}

// http://pages.cs.wisc.edu/~cao/papers/summary-cache/node8.html
func NewBloomFilter(bits uint64) *BloomFilter {
	filter := &BloomFilter{}
	filter.m = bits
	filter.k = 7
	filter.keys = make([]byte, bits)
	return filter
}

func LoadBloomFilter(reader io.Reader) (*BloomFilter, error) {
	rawM := make([]byte, 4)
	_, err := io.ReadFull(reader, rawM)
	if err != nil {
		return nil, err
	}
	rawN := make([]byte, 4)
	_, err = io.ReadFull(reader, rawN)
	if err != nil {
		return nil, err
	}
	rawK := make([]byte, 1)
	_, err = io.ReadFull(reader, rawK)
	if err != nil {
		return nil, err
	}
	keys, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	filter := &BloomFilter{
		m:    binary.BigEndian.Uint64(rawM),
		n:    binary.BigEndian.Uint64(rawN),
		k:    rawK[0],
		keys: keys,
	}
	return filter, nil
}

func (f *BloomFilter) Add(data []byte) {
	f.lock.Lock()
	defer f.lock.Unlock()

	locations := f.getLocations(data)
	for _, loc := range locations {
		slot := loc / bitsPerByte
		mod := loc % bitsPerByte
		f.keys[slot] |= 1 << mod
	}

	f.n++
}

func (f *BloomFilter) Exists(data []byte) bool {
	f.lock.RLock()
	defer f.lock.RUnlock()

	locations := f.getLocations(data)
	for _, loc := range locations {
		slot := loc / bitsPerByte
		mod := loc % bitsPerByte
		if f.keys[slot]&(1<<mod) == 0 {
			return false
		}
	}

	return true
}

func (f *BloomFilter) Save(writer io.Writer) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	header := make([]byte, 0)
	binary.BigEndian.AppendUint64(header, f.m)
	binary.BigEndian.AppendUint64(header, f.n)
	header = append(header, f.k)
	_, err := writer.Write(header)
	if err != nil {
		return err
	}
	_, err = writer.Write(f.keys)
	if err != nil {
		return err
	}
	return nil
}

func (f *BloomFilter) getLocations(data []byte) []uint {
	locations := make([]uint, f.k)
	for i := uint(0); i < uint(f.k); i++ {
		hashValue := baseHash(append(data, byte(i)))
		locations[i] = uint(hashValue % uint64(f.m))
	}

	return locations
}

func baseHash(data []byte) uint64 {
	hasher := murmur3.New64()
	hasher.Write(data)
	return hasher.Sum64()
}
