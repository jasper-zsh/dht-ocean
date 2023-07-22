package util

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBloom(t *testing.T) {
	filter := NewBloomFilter(1024 * 1024)
	filter.Add([]byte{123})
	file, err := os.Create("bloom.test")
	assert.NoError(t, err)
	err = filter.Save(file)
	assert.NoError(t, err)
	file.Close()
	file, err = os.Open("bloom.test")
	assert.NoError(t, err)
	filter, err = LoadBloomFilter(file)
	assert.NoError(t, err)
	file.Close()
	r := filter.Exists([]byte{123})
	assert.True(t, r)
	r = filter.Exists([]byte{99})
	assert.False(t, r)
}
