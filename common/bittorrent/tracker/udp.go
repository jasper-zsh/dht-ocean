package tracker

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"time"
)

const (
	protocolID    uint64 = 0x41727101980
	actionConnect uint32 = 0
	actionScrape  uint32 = 2
	actionError   uint32 = 3
)

var _ Tracker = (*UDPTracker)(nil)

type UDPTracker struct {
	addr         *net.UDPAddr
	conn         *net.UDPConn
	connectionID uint64
}

func NewUDPTracker(addr string) (*UDPTracker, error) {
	a, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	t := &UDPTracker{
		addr: a,
	}
	return t, nil
}

func (t *UDPTracker) Start() {
	c, err := net.DialUDP("udp", nil, t.addr)
	if err != nil {
		panic(err)
	}
	t.conn = c
	err = t.connect()
	if err != nil {
		panic(err)
	}
}

func (t *UDPTracker) Stop() {
	if t.conn != nil {
		err := t.conn.Close()
		if err != nil {
			panic(err)
		}
		t.conn = nil
	}
}

func (t *UDPTracker) connect() error {
	buf := make([]byte, 16)
	tid := rand.Uint32()
	binary.BigEndian.PutUint64(buf, protocolID)
	binary.BigEndian.PutUint32(buf[8:], actionConnect)
	binary.BigEndian.PutUint32(buf[12:], tid)
	_, err := t.conn.Write(buf)
	if err != nil {
		return err
	}
	resp, err := t.readUntilTid(tid, time.Second*10)
	if err != nil {
		return err
	}
	if binary.BigEndian.Uint32(resp[0:]) != 0 {
		return fmt.Errorf("illegal connect response")
	}
	t.connectionID = binary.BigEndian.Uint64(resp[8:])
	return nil
}

func (t *UDPTracker) readUntilTid(tid uint32, timeout time.Duration) ([]byte, error) {
	timeoutAt := time.Now().Add(timeout)
	_ = t.conn.SetReadDeadline(timeoutAt)

	buf := make([]byte, 4096)
	for {
		bytes, _, err := t.conn.ReadFromUDP(buf)
		if err != nil {
			return nil, err
		}
		if binary.BigEndian.Uint32(buf[4:]) == tid {
			return buf[:bytes], nil
		}
		if timeoutAt.Before(time.Now()) {
			return nil, fmt.Errorf("timeout")
		}
	}
}

func (t *UDPTracker) Scrape(infoHashes [][]byte) ([]*ScrapeResponse, error) {
	buf := make([]byte, 16+20*len(infoHashes))
	tid := rand.Uint32()
	binary.BigEndian.PutUint64(buf, t.connectionID)
	binary.BigEndian.PutUint32(buf[8:], actionScrape)
	binary.BigEndian.PutUint32(buf[12:], tid)
	for i, infoHash := range infoHashes {
		copy(buf[16+20*i:], infoHash[:20])
	}
	_, err := t.conn.Write(buf)
	if err != nil {
		return nil, err
	}
	resp, err := t.readUntilTid(tid, time.Second*10)
	if err != nil {
		return nil, err
	}
	if binary.BigEndian.Uint32(resp) == actionError {
		return nil, fmt.Errorf("error resp: %s", resp[8:])
	}
	if binary.BigEndian.Uint32(resp) != actionScrape {
		return nil, fmt.Errorf("illegal scrape response")
	}
	respCount := (len(resp) - 8) / 12
	resps := make([]*ScrapeResponse, respCount)
	for i := 0; i < respCount; i++ {
		r := &ScrapeResponse{}
		r.Seeders = binary.BigEndian.Uint32(resp[8+12*i:])
		r.Completed = binary.BigEndian.Uint32(resp[12+12*i:])
		r.Leechers = binary.BigEndian.Uint32(resp[16+12*i:])
		resps[i] = r
	}
	return resps, nil
}
