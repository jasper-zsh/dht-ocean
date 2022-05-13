package dht

import (
	"crypto/sha1"
	"math/rand"
	"strconv"
)

func GenerateNodeID() []byte {
	hash := sha1.New()
	for i := 0; i < 32; i++ {
		hash.Write([]byte(strconv.Itoa(rand.Int())))
	}
	return hash.Sum(nil)
}
