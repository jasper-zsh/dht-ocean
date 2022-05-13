package main

import (
	"dht-ocean/dht"
	"dht-ocean/dht/protocol"
	"fmt"
	"github.com/sirupsen/logrus"
	"net"
	"os"
)

func AddAddrToChannel(ch chan *net.UDPAddr, addr string) {
	a, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		logrus.Errorf("Failed to resolve addr: %s", addr)
		return
	}
	ch <- a
}

func main() {
	logrus.SetLevel(logrus.DebugLevel)

	nodes := make(map[string]*protocol.Node)
	addrsToSniff := make(chan *net.UDPAddr, 20)

	AddAddrToChannel(addrsToSniff, "dht.transmissionbt.com:6881")
	//addrsToSniff <- &protocol.Node{Addr: "dht.transmissionbt.com", Port: 6881}
	//addrsToSniff <- &protocol.Node{Addr: "router.bittorrent.com", Port: 6881}
	//addrsToSniff <- &protocol.Node{Addr: "router.utorrent.com", Port: 6881}

	nodeID, err := os.ReadFile("node_id")
	if err != nil {
		logrus.Warnf("Cannot read nodeID, generated randomly.")
		nodeID = protocol.GenerateNodeID()
		err = os.WriteFile("node_id", nodeID, 0666)
		if err != nil {
			logrus.Errorf("Failed to write nodeID")
		}
	}
	server, err := dht.NewDHT(":6881", nodeID)
	defer server.Stop()
	if err != nil {
		logrus.Errorf("Failed to start dht server. %v", err)
		panic(err)
	}
	server.RegisterFindNodeHandler(func(response *protocol.FindNodeResponse) error {
		for _, node := range response.Nodes {
			id := string(node.NodeID)
			_, ok := nodes[id]
			if !ok {
				nodes[id] = node
				addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", node.Addr, node.Port))
				if err != nil {
					return err
				}
				addrsToSniff <- addr
			}
		}
		logrus.Infof("Found %d nodes Total %d nodes Queued %d nodes.", len(response.Nodes), len(nodes), len(addrsToSniff))
		return nil
	})

	server.Run()

	for {
		addr := <-addrsToSniff
		err := server.FindNode(protocol.GenerateNodeID(), addr)
		if err != nil {
			logrus.Errorf("Failed to send find_node query. %v", err)
		}
	}
}
