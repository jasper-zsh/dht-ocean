package main

import (
	"dht-ocean/dht"
	"dht-ocean/dht/protocol"
	"fmt"
)

func main() {
	nodes := make(map[string]*protocol.Node)
	conn, err := protocol.NewDHTConn("dht.transmissionbt.com:6881", dht.GenerateNodeID())
	//conn, err := protocol.NewDHTConn("dht.transmissionbt.com:6881", []byte("0123456789abcdefghij"))
	defer conn.Close()
	if err != nil {
		panic(err)
	}

	nodesToSniff := make(chan string, 20)

	fmt.Println("Sending initial find_node query...")

	err = conn.FindNode(dht.GenerateNodeID())
	if err != nil {
		fmt.Printf("ERROR: Failed to send initial find_node query. %s\n", err.Error())
		return
	}

	handleNodes := func(list []*protocol.Node) {
		fmt.Printf("Found %d nodes total %d.\n", len(list), len(nodes))
		for _, node := range list {
			id := string(node.NodeID)
			_, ok := nodes[id]
			if !ok {
				nodes[id] = node
				nodesToSniff <- id
			}
		}
	}

	handleFindNode := func(pkt *dht.Packet) {
		r, err := protocol.NewFindNodeResponse(pkt)
		if err != nil {
			fmt.Printf("ERROR: Failed to parse find_node response. %s", err.Error())
			return
		}
		handleNodes(r.Nodes)
	}

	pkt, err := conn.ReadPacket()
	if err != nil {
		fmt.Printf("ERROR: Failed to parse initial find_node response. %s\n", err.Error())
		return
	}
	handleFindNode(pkt)

	for {
		id := <-nodesToSniff
		node := nodes[id]
		go func() {
			err := node.Connect()
			defer node.Disconnect()
			if err != nil {
				fmt.Printf("Warn: Failed to connect to node %x. %s\n", node.NodeID, err.Error())
				return
			}

			r, err := node.FindNode(nil)
			if err != nil {
				fmt.Printf("Warn: Failed to find_node from %x. %s\n", node.NodeID, err.Error())
				return
			}
			handleNodes(r.Nodes)
		}()
	}
}
