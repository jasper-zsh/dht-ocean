package util

import (
	"encoding/binary"
	"io"
	"sync"

	"github.com/RoaringBitmap/roaring/roaring64"
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
	keys *roaring64.Bitmap
}

// http://pages.cs.wisc.edu/~cao/papers/summary-cache/node8.html
func NewBloomFilter(bits uint64) *BloomFilter {
	filter := &BloomFilter{}
	filter.m = bits
	filter.k = 7
	filter.keys = roaring64.NewBitmap()
	return filter
}

func LoadBloomFilter(reader io.Reader) (*BloomFilter, error) {
	rawM := make([]byte, 8)
	_, err := io.ReadFull(reader, rawM)
	if err != nil {
		return nil, err
	}
	rawN := make([]byte, 8)
	_, err = io.ReadFull(reader, rawN)
	if err != nil {
		return nil, err
	}
	rawK := make([]byte, 1)
	_, err = io.ReadFull(reader, rawK)
	if err != nil {
		return nil, err
	}
	filter := &BloomFilter{
		m: binary.BigEndian.Uint64(rawM),
		n: binary.BigEndian.Uint64(rawN),
		k: rawK[0],
	}
	filter.keys = roaring64.NewBitmap()
	_, err = filter.keys.ReadFrom(reader)
	if err != nil {
		return nil, err
	}
	return filter, nil
}

func (f *BloomFilter) Add(data []byte) {
	f.lock.Lock()
	defer f.lock.Unlock()

	locations := f.getLocations(data)
	for _, loc := range locations {
		f.keys.Add(loc)
	}

	f.n++
}

func (f *BloomFilter) Exists(data []byte) bool {
	f.lock.RLock()
	defer f.lock.RUnlock()

	locations := f.getLocations(data)
	for _, loc := range locations {
		if !f.keys.Contains(loc) {
			return false
		}
	}

	return true
}

func (f *BloomFilter) Save(writer io.Writer) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	header := make([]byte, 0)
	header = binary.BigEndian.AppendUint64(header, f.m)
	header = binary.BigEndian.AppendUint64(header, f.n)
	header = append(header, f.k)
	_, err := writer.Write(header)
	if err != nil {
		return err
	}
	_, err = f.keys.WriteTo(writer)
	if err != nil {
		return err
	}
	return nil
}

func (f *BloomFilter) getLocations(data []byte) []uint64 {
	locations := make([]uint64, f.k)
	for i := uint64(0); i < uint64(f.k); i++ {
		hashValue := baseHash(append(data, byte(i)))
		locations[i] = hashValue % uint64(f.m)
	}

	return locations
}

func baseHash(data []byte) uint64 {
	hasher := murmur3.New64()
	hasher.Write(data)
	return hasher.Sum64()
}
