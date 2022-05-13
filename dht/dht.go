package dht

import (
	"github.com/sirupsen/logrus"
	"net"
)

type DHT struct {
	conn *net.UDPConn
}

func NewDHT(addr string) (*DHT, error) {
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return nil, err
	}
	dht := &DHT{
		conn: conn.(*net.UDPConn),
	}
	return dht, nil
}

func (dht *DHT) Run() {
	go dht.listen()
}

func (dht *DHT) send(data []byte, addr *net.UDPAddr) error {
	sent, err := dht.conn.WriteToUDP(data, addr)
	if err != nil {
		return err
	}
	logrus.Debugf("Sent %d bytes to %s", sent, addr)
	return nil
}

func (dht *DHT) sendPacket(pkt *Packet, addr *net.UDPAddr) error {
	encoded, err := pkt.Encode()
	if err != nil {
		return err
	}
	return dht.send([]byte(encoded), addr)
}

func (dht *DHT) FindNode(addr *net.UDPAddr) {

}

func (dht *DHT) listen() {
	buf := make([]byte, 2048)
	for {
		n, addr, err := dht.conn.ReadFromUDP(buf)
		if err != nil {
			logrus.Warnf("Failed to read from udp. %v", err)
			continue
		}
		logrus.Debugf("Received %d bytes from udp %s", n, addr)
	}
}
