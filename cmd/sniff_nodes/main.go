package main

import (
	"dht-ocean/dht"
	"dht-ocean/dht/protocol"
	"fmt"
)

func main() {
	nodes := make(map[string]*protocol.Node)
	conn, err := protocol.NewDHTConn("dht.transmissionbt.com:6881", dht.GenerateNodeID())
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	nodesToSniff := make(chan string, 20)

	fmt.Println("Sending ping query...")
	err = conn.Ping()
	if err != nil {
		fmt.Printf("ERROR: Failed to send ping query. %s\n", err.Error())
		return
	}
	pkt, err := conn.ReadPacket()
	if err != nil {
		fmt.Printf("ERROR: Failed to handle ping response. %s\n", err.Error())
		return
	}
	pkt.Print()
	pong, err := protocol.NewPingResponse(pkt)
	if err != nil {
		fmt.Printf("ERROR: Failed to parse ping response. %s\n", err.Error())
		return
	}

	err = conn.FindNode(pong.NodeID)
	if err != nil {
		fmt.Printf("ERROR: Failed to send initial find_node query. %s\n", err.Error())
		return
	}

	handleFindNode := func(pkt *dht.Packet) {
		r, err := protocol.NewFindNodeResponse(pkt)
		if err != nil {
			fmt.Printf("ERROR: Failed to parse find_node response. %s", err.Error())
			return
		}
		fmt.Printf("Found %d nodes total %d.\n", len(r.Nodes), len(nodes))
		for _, node := range r.Nodes {
			id := string(node.NodeID)
			_, ok := nodes[id]
			if !ok {
				nodes[id] = node
				nodesToSniff <- id
			}
		}
	}

	pkt, err = conn.ReadPacket()
	if err != nil {
		fmt.Printf("ERROR: Failed to parse initial find_node response. %s\n", err.Error())
		return
	}
	handleFindNode(pkt)

	for {
		id := <-nodesToSniff
		err := conn.FindNode([]byte(id))
		if err != nil {
			fmt.Printf("ERROR: Failed to send find_node query to %x. %s\n", id, err.Error())
			continue
		}
		pkt, err = conn.ReadPacket()
		if err != nil {
			fmt.Printf("ERROR: Failed to read find_node query to %x. %s\n", id, err.Error())
			continue
		}
		handleFindNode(pkt)
	}
}
