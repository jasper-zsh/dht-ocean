package main

import (
	"dht-ocean/dht/protocol"
	"fmt"
)

func main() {
	nodes := make(map[string]*protocol.Node)
	nodesToSniff := make(chan *protocol.Node, 20)

	nodesToSniff <- &protocol.Node{Addr: "dht.transmissionbt.com", Port: 6881}
	nodesToSniff <- &protocol.Node{Addr: "router.bittorrent.com", Port: 6881}
	nodesToSniff <- &protocol.Node{Addr: "router.utorrent.com", Port: 6881}

	handleNodes := func(list []*protocol.Node) {
		fmt.Printf("Found %d nodes total %d.\n", len(list), len(nodes))
		for _, node := range list {
			id := string(node.NodeID)
			_, ok := nodes[id]
			if !ok {
				nodes[id] = node
				nodesToSniff <- node
			}
		}
	}

	for {
		node := <-nodesToSniff
		go func() {
			err := node.Connect()
			defer node.Disconnect()
			if err != nil {
				fmt.Printf("Warn: Failed to connect to node %x. %s\n", node.NodeID, err.Error())
				return
			}

			r, err := node.FindNode(nil)
			if err != nil {
				fmt.Printf("Warn: Failed to find_node from %s:%d. %s\n", node.Addr, node.Port, err.Error())
				return
			}
			handleNodes(r.Nodes)
		}()
	}
}
