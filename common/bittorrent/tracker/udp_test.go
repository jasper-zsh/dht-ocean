package tracker

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUDPTracker_Scrape(t *testing.T) {
	tracker, err := NewUDPTracker(context.Background(), "tracker.openbittorrent.com:6969")
	if !assert.NoError(t, err) {
		return
	}
	tracker.Start()
	if !assert.NoError(t, err) {
		return
	}
	defer tracker.Stop()

	infoHashes := make([][]byte, 3)
	infoHashes[0], _ = hex.DecodeString("d5c633aed4c3fe873a7cdb43c1bba38614fb68a4")
	infoHashes[1], _ = hex.DecodeString("a0118b9300a5d1fce5e95ad1a15bd525bc88ba0a")
	infoHashes[2], _ = hex.DecodeString("197f5e159890de15ce9db9b7aa3044da1078ab2a")
	err = tracker.Scrape(infoHashes)
	if !assert.NoError(t, err) {
		return
	}
	// for _, result := range results {
	// 	fmt.Printf("Seeders %d Completed %d Leechers %d\n", result.Seeders, result.Completed, result.Leechers)
	// }
}
