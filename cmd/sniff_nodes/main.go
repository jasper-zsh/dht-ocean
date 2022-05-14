package main

import (
	"dht-ocean/crawler"
	"dht-ocean/dht/protocol"
	"github.com/sirupsen/logrus"
	"os"
	"time"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)

	nodeID, err := os.ReadFile("node_id")
	if err != nil {
		logrus.Warnf("Cannot read nodeID, generated randomly.")
		nodeID = protocol.GenerateNodeID()
		err = os.WriteFile("node_id", nodeID, 0666)
		if err != nil {
			logrus.Errorf("Failed to write nodeID")
		}
	}

	c, err := crawler.NewCrawler(":6881", nodeID)
	if err != nil {
		logrus.Errorf("Failed to create crawler. %v", err)
		panic(err)
	}
	c.SetBootstrapNodes([]string{
		"dht.transmissionbt.com:6881",
		"tracker.moeking.me:6881",
		"router.bittorrent.com:6881",
		"router.utorrent.com:6881",
		"dht.aelitis.com:6881",
	})
	if c.Run() != nil {
		logrus.Errorf("Failed to run crawler. %v", err)
	}
	defer c.Stop()

	for {
		time.Sleep(time.Second)
		c.LogStats()
	}
}
