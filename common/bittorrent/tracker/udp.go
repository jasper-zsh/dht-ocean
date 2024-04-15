package tracker

import (
	"bytes"
	"context"
	"dht-ocean/common/util"
	"encoding/binary"
	"io"
	"math/rand"
	"net"

	"github.com/juju/errors"
	"github.com/zeromicro/go-zero/core/logx"
)

var _ Tracker = (*UDPTracker)(nil)

type UDPTracker struct {
	addr         string
	conn         *net.UDPConn
	connected    chan struct{}
	connectionID uint64

	scrapeQueue *util.LRWCache[uint32, [][]byte]

	result chan []*ScrapeResult
}

// Result implements Tracker.
func (t *UDPTracker) Result() chan []*ScrapeResult {
	return t.result
}

// ScrapeAsync implements Tracker.
func (t *UDPTracker) Scrape(infoHashes [][]byte) error {
	if t.connectionID == 0 {
		<-t.connected
	}
	writer := bytes.NewBuffer(make([]byte, 0, 16+20*len(infoHashes)))
	reqHdr := TrackerRequestHeader{
		ConnectionID:  t.connectionID,
		Action:        ActionScrape,
		TransactionID: rand.Uint32(),
	}
	err := binary.Write(writer, binary.BigEndian, reqHdr)
	if err != nil {
		return errors.Trace(err)
	}
	for _, infoHash := range infoHashes {
		_, err = writer.Write(infoHash[:20])
		if err != nil {
			return errors.Trace(err)
		}
	}
	t.scrapeQueue.Set(reqHdr.TransactionID, infoHashes)
	_, err = t.conn.Write(writer.Bytes())
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

func NewUDPTracker(ctx context.Context, addr string) (*UDPTracker, error) {
	t := &UDPTracker{
		addr:        addr,
		connected:   make(chan struct{}),
		scrapeQueue: util.NewLRWCache[uint32, [][]byte](ctx, 10, 50, true),
		result:      make(chan []*ScrapeResult),
	}
	return t, nil
}

func (t *UDPTracker) Start() error {
	if t.conn != nil {
		return nil
	}
	addr, err := net.ResolveUDPAddr("udp", t.addr)
	if err != nil {
		return errors.Trace(err)
	}
	c, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return errors.Trace(err)
	}
	t.conn = c
	if t.connectionID == 0 {
		err = t.sendConnect()
		if err != nil {
			return errors.Trace(err)
		}
	}
	go t.receive()
	return nil
}

func (t *UDPTracker) Stop() {
	if t.conn != nil {
		t.conn.Close()
		t.conn = nil
	}
}

func (t *UDPTracker) disconnect() {
	if t.conn != nil {
		t.conn.Close()
		t.conn = nil
	}
}

func (t *UDPTracker) receive() {
	hdr := TrackerResponseHeader{}
	// Largest scrape response is 16 + 20*n (max n is 74)
	// 2048 is enough
	buf := make([]byte, 2048)
	for {
		n, _, err := t.conn.ReadFrom(buf)
		if err != nil {
			logx.Errorf("failed to read: %+v", err)
			t.disconnect()
			return
		}
		reader := bytes.NewReader(buf[:n])
		err = binary.Read(reader, binary.BigEndian, &hdr)
		if err != nil {
			logx.Errorf("failed to parse header: %+v", err)
			t.disconnect()
			return
		}
		switch hdr.Action {
		case ActionConnect:
			err = t.handleConnect(reader)
		case ActionScrape:
			err = t.handleScrape(reader, &hdr)
		default:
			err = errors.Errorf("unknown action: %d", hdr.Action)
		}
		if err != nil {
			t.disconnect()
			return
		}
	}
}

func (t *UDPTracker) handleConnect(reader io.Reader) error {
	resp := ConnectResponse{}
	err := binary.Read(reader, binary.BigEndian, &resp)
	if err != nil {
		return errors.Trace(err)
	}
	t.connectionID = resp.ConnectionID
	logx.Infof("Connected: %d", resp.ConnectionID)
	if len(t.connected) == 0 {
		t.connected <- struct{}{}
	}
	return nil
}

func (t *UDPTracker) sendConnect() error {
	req := ConnectRequest{
		ProtocolID:    ProtocolID,
		Action:        ActionConnect,
		TransactionID: rand.Uint32(),
	}
	err := binary.Write(t.conn, binary.BigEndian, req)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (t *UDPTracker) handleScrape(reader io.Reader, hdr *TrackerResponseHeader) error {
	infoHashes, ok := t.scrapeQueue.GetAndRemove(hdr.TransactionID)
	if !ok {
		logx.Infof("transaction %d lost", hdr.TransactionID)
		return nil
	}
	results := make([]*ScrapeResult, 0, len(infoHashes))
	for _, infoHash := range infoHashes {
		r := ScrapeResult{
			InfoHash: infoHash,
		}
		err := binary.Read(reader, binary.BigEndian, &r.ScrapeResponse)
		if err != nil {
			return errors.Trace(err)
		}
		results = append(results, &r)
	}
	t.result <- results
	return nil
}
