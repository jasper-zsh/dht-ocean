package main

import (
	"dht-ocean/dht"
	"dht-ocean/dht/protocol"
	"github.com/sirupsen/logrus"
	"os"
)

func AddNodeToChannel(ch chan *protocol.Node, nodeID []byte, addr string, port int) {
	ch <- &protocol.Node{
		NodeID: nodeID,
		Addr:   addr,
		Port:   port,
	}
}

func main() {
	logrus.SetLevel(logrus.DebugLevel)

	nodes := make(map[string]*protocol.Node)
	NodesToSniff := make(chan *protocol.Node, 20)

	nodeID, err := os.ReadFile("node_id")
	if err != nil {
		logrus.Warnf("Cannot read nodeID, generated randomly.")
		nodeID = protocol.GenerateNodeID()
		err = os.WriteFile("node_id", nodeID, 0666)
		if err != nil {
			logrus.Errorf("Failed to write nodeID")
		}
	}
	AddNodeToChannel(NodesToSniff, nodeID, "dht.transmissionbt.com", 6881)
	AddNodeToChannel(NodesToSniff, nodeID, "router.bittorrent.com", 6881)
	AddNodeToChannel(NodesToSniff, nodeID, "router.utorrent.com", 6881)

	server, err := dht.NewDHT(":6881", nodeID)
	defer server.Stop()
	if err != nil {
		logrus.Errorf("Failed to start dht server. %v", err)
		panic(err)
	}
	server.RegisterFindNodeHandler(func(response *protocol.FindNodeResponse) error {
		newNodes := make([]*protocol.Node, 0, len(response.Nodes))
		for _, node := range response.Nodes {
			id := string(node.NodeID)
			_, ok := nodes[id]
			if !ok {
				newNodes = append(newNodes, node)
			}
		}
		for _, node := range newNodes {
			id := string(node.NodeID)
			nodes[id] = node
			NodesToSniff <- node
		}
		logrus.Infof("Found %d new nodes Total %d nodes Queued %d nodes.", len(newNodes), len(nodes), len(NodesToSniff))
		return nil
	})

	server.Run()

	for {
		node := <-NodesToSniff
		err := server.FindNode(node, protocol.GenerateNodeID())
		if err != nil {
			logrus.Errorf("Failed to send find_node query. %v", err)
		}
	}
}
