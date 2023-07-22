package utils

import (
	"encoding/json"
	"io"
	"sync"

	"github.com/spaolacci/murmur3"
)

const (
	bitsPerByte = 8
	mod7        = 1<<3 - 1
)

type BloomFilter struct {
	lock sync.RWMutex `json:"-"`

	payload bloomPayload
}

type bloomPayload struct {
	M    uint64 `json:"m"`
	N    uint64 `json:"n"`
	K    uint   `json:"k"`
	Keys []byte `json:"keys"`
}

func NewBloomFilter(bits uint64) *BloomFilter {
	filter := &BloomFilter{}
	filter.payload.M = bits
	filter.payload.K = 13
	filter.payload.Keys = make([]byte, bits)
	return filter
}

func LoadBloomFilter(reader io.Reader) (*BloomFilter, error) {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	filter := &BloomFilter{}
	err = json.Unmarshal(raw, &filter.payload)
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
		slot := loc / bitsPerByte
		mod := loc % bitsPerByte
		f.payload.Keys[slot] |= 1 << mod
	}

	f.payload.N++
}

func (f *BloomFilter) Exists(data []byte) bool {
	f.lock.RLock()
	defer f.lock.RUnlock()

	locations := f.getLocations(data)
	for _, loc := range locations {
		slot := loc / bitsPerByte
		mod := loc % bitsPerByte
		if f.payload.Keys[slot]&(1<<mod) == 0 {
			return false
		}
	}

	return true
}

func (f *BloomFilter) Save(writer io.Writer) error {
	raw, err := json.Marshal(f.payload)
	if err != nil {
		return err
	}
	_, err = writer.Write(raw)
	if err != nil {
		return err
	}
	return nil
}

func (f *BloomFilter) getLocations(data []byte) []uint {
	locations := make([]uint, f.payload.K)
	for i := uint(0); i < uint(f.payload.K); i++ {
		hashValue := baseHash(append(data, byte(i)))
		locations[i] = uint(hashValue % uint64(f.payload.M))
	}

	return locations
}

func baseHash(data []byte) uint64 {
	hasher := murmur3.New64()
	hasher.Write(data)
	return hasher.Sum64()
}
