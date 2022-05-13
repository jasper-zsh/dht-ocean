package dht

import (
	"dht-ocean/dht/protocol"
	"fmt"
	"github.com/sirupsen/logrus"
	"net"
	"time"
	"unsafe"
)

type FindNodeHandler func(response *protocol.FindNodeResponse) error

type DHT struct {
	conn               *net.UDPConn
	nodeID             []byte
	nextTransactionID  uint16
	findNodeHandlers   []FindNodeHandler
	transactionStorage TransactionStorage
}

func NewDHT(addr string, nodeID []byte) (*DHT, error) {
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return nil, err
	}
	dht := &DHT{
		conn:               conn.(*net.UDPConn),
		nodeID:             nodeID,
		transactionStorage: make(TransactionStorage),
	}
	return dht, nil
}

func (dht *DHT) Run() {
	go dht.listen()
}

func (dht *DHT) Stop() {
	_ = dht.conn.Close()
}

func (dht *DHT) RegisterFindNodeHandler(handler FindNodeHandler) {
	dht.findNodeHandlers = append(dht.findNodeHandlers, handler)
}

func (dht *DHT) send(data []byte, addr *net.UDPAddr) error {
	_ = dht.conn.SetWriteDeadline(time.Now().Add(time.Second * 10))
	sent, err := dht.conn.WriteToUDP(data, addr)
	if err != nil {
		return err
	}
	logrus.Debugf("Sent %d bytes to %s", sent, addr)
	return nil
}

func (dht *DHT) sendPacket(pkt *protocol.Packet, addr *net.UDPAddr) error {
	encoded, err := pkt.Encode()
	if err != nil {
		return err
	}
	logrus.Debugf("Send packet with Tid %X", pkt.GetT())
	return dht.send([]byte(encoded), addr)
}

func (dht *DHT) nextTransaction() []byte {
	tid := *(*[2]byte)(unsafe.Pointer(&dht.nextTransactionID))
	dht.nextTransactionID += 1
	return []byte{tid[0], tid[1]}
}

func (dht *DHT) FindNode(node *protocol.Node, target []byte) error {
	req := protocol.NewFindNodeRequest(dht.nodeID, target)
	tid := dht.nextTransaction()
	dht.transactionStorage.Add(&TransactionContext{
		Tid:       tid,
		QueryType: "find_node",
	})

	req.SetT(tid)
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", node.Addr, node.Port))
	if err != nil {
		return err
	}
	return dht.sendPacket(req.Packet, addr)
}

func (dht *DHT) listen() {
	for {
		buf := make([]byte, 2048)
		n, addr, err := dht.conn.ReadFromUDP(buf)
		if err != nil {
			logrus.Warnf("Failed to read from udp. %v", err)
			continue
		}
		logrus.Debugf("Received %d bytes from udp %s", n, addr)
		pkt, err := protocol.NewPacketFromBuffer(buf)
		pkt.Addr = addr
		dht.handle(pkt)
	}
}

func (dht *DHT) handle(pkt *protocol.Packet) {
	switch pkt.GetY() {
	case "q":
		q := pkt.Get("q")
		if q == nil {
			logrus.Warnf("Illegal incoming query with no query type.")
			pkt.Print()
			return
		}
		switch string(q.([]byte)) {
		case "ping":
			r := protocol.NewPingResponse(dht.nodeID)
			r.SetT(pkt.GetT())
			err := dht.sendPacket(pkt, pkt.Addr)
			if err != nil {
				logrus.Warnf("Failed to response a ping. %v", err)
			}
		default:
			logrus.Warnf("Unhandled query: %s", q)
			pkt.Print()
		}
	case "r":
		tid := pkt.GetT()
		ctx := dht.transactionStorage.Get(tid)
		if ctx == nil {
			logrus.Warnf("Transaction %X not found, skip handlers.", tid)
			pkt.Print()
			return
		}
		switch ctx.QueryType {
		case "find_node":
			res, err := protocol.NewFindNodeResponse(pkt)
			if err != nil {
				logrus.Warnf("Failed to parse find_node response.")
				pkt.Print()
			}
			for _, handler := range dht.findNodeHandlers {
				err := handler(res)
				if err != nil {
					logrus.Warnf("Failed to handle find_node response. %v", err)
				}
			}
		default:
			logrus.Warnf("Unknown response: %s", pkt.Get("q"))
			pkt.Print()
		}
	default:
		logrus.Warnf("Unknown packet: %s", pkt.GetY())
		pkt.Print()
	}
}
