package bittorrent

import (
	"bytes"
	"crypto/sha1"
	bencode2 "dht-ocean/common/bencode"
	"dht-ocean/common/dht"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/proxy"
)

var (
	btProtocol           = []byte("BitTorrent protocol")
	btReserved           = []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x10, 0x00, 0x01}
	pieceLength          = int(math.Pow(2, 14))
	maxMetadataSize      = 10000000
	extHandshakeID  byte = 0
	btMsgID         byte = 20
)

type TrafficMetricFunc func(label string, length int)

type ExtHandshakeResult struct {
	MetadataSize int
	UtMetadata   int
	NumPieces    int
}

type Piece struct {
	Data []byte
	ID   int
}

type BitTorrent struct {
	Proxy             proxy.Dialer
	Addr              string
	conn              net.Conn
	InfoHash          []byte
	nodeID            []byte
	trafficMetricFunc TrafficMetricFunc
}

func NewBitTorrent(nodeID, infoHash []byte, addr string) *BitTorrent {
	r := &BitTorrent{
		Addr:     addr,
		InfoHash: infoHash,
		nodeID:   nodeID,
	}
	return r
}

func (bt *BitTorrent) SetTrafficMetricFunc(f TrafficMetricFunc) {
	bt.trafficMetricFunc = f
}

func (bt *BitTorrent) trafficMetric(label string, length int) {
	if bt.trafficMetricFunc != nil {
		bt.trafficMetricFunc(label, length)
	}
}

func (bt *BitTorrent) Start() error {
	var err error
	if bt.Proxy != nil {
		bt.conn, err = bt.Proxy.Dial("tcp", bt.Addr)
		if err != nil {
			return errors.WithStack(err)
		}
	} else {
		bt.conn, err = net.DialTimeout("tcp", bt.Addr, time.Second*10)
		if err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func (bt *BitTorrent) Stop() error {
	err := bt.conn.Close()
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (bt *BitTorrent) sendMessage(msg []byte) error {
	lenB := make([]byte, 4)
	binary.BigEndian.PutUint32(lenB, uint32(len(msg)))
	_, err := bt.write(lenB)
	if err != nil {
		return err
	}
	bt.trafficMetric("out_bt_message_header", len(lenB))
	_, err = bt.write(msg)
	if err != nil {
		return err
	}
	return nil
}

func (bt *BitTorrent) write(data []byte) (int, error) {
	_ = bt.conn.SetWriteDeadline(time.Now().Add(time.Second * 10))
	return bt.conn.Write(data)
}

func (bt *BitTorrent) read(data []byte) (int, error) {
	_ = bt.conn.SetReadDeadline(time.Now().Add(time.Second * 10))
	return io.ReadFull(bt.conn, data)
}

func (bt *BitTorrent) GetMetadata() (metadata map[string]any, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("panic %v", err)
		}
	}()
	err = bt.handshake()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	ext, err := bt.extHandshake()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	pieces, err := bt.requestPieces(ext)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	rawMetadata := make([]byte, ext.MetadataSize)
	for _, piece := range pieces {
		copy(rawMetadata[piece.ID*pieceLength:], piece.Data)
	}
	h := sha1.New()
	h.Write(rawMetadata)
	hash := h.Sum(nil)
	if !bytes.Equal(bt.InfoHash, hash) {
		return nil, fmt.Errorf("corrupt torrent")
	}
	metadata, _, err = bencode2.BDecodeDict(rawMetadata)
	if err != nil {
		return nil, err
	}
	return metadata, nil
}

func (bt *BitTorrent) handshake() error {
	pkt := make([]byte, 1)
	pkt[0] = byte(len(btProtocol))
	pkt = append(pkt, btProtocol...)
	pkt = append(pkt, btReserved...)
	pkt = append(pkt, bt.InfoHash...)
	// AlphaReign
	pkt = append(pkt, dht.GenerateNodeID()...)
	// Official
	//pkt = append(pkt, bt.nodeID...)
	_, err := bt.write(pkt)
	if err != nil {
		return errors.WithStack(err)
	}
	bt.trafficMetric("out_bt_handshake", len(pkt))
	pLength := make([]byte, 1)
	b, err := bt.read(pLength)
	if err != nil {
		return errors.WithStack(err)
	}
	if b == 0 {
		return fmt.Errorf("read empty protocol length")
	}
	dataLen := int(pLength[0]) + 48
	data := make([]byte, dataLen)
	_, err = bt.read(data)
	if err != nil {
		return errors.WithStack(err)
	}
	bt.trafficMetric("in_bt_handshake", len(data)+len(pLength))
	if !bytes.Equal(data[:pLength[0]], btProtocol) {
		return fmt.Errorf("not bt protocol")
	}
	data = data[pLength[0]:]
	if data[5]&0x10 > 0 {
		return nil
	} else {
		return fmt.Errorf("handshake failed")
	}
}

func (bt *BitTorrent) extHandshake() (*ExtHandshakeResult, error) {
	p, err := bencode2.BEncode(map[string]any{
		"m": map[string]any{
			"ut_metadata": 1,
		},
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	msg := append([]byte{btMsgID, extHandshakeID}, []byte(p)...)
	err = bt.sendMessage(msg)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	bt.trafficMetric("out_bt_ext_handshake", len(msg))
	msg, err = bt.readExtMessage()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	bt.trafficMetric("in_bt_ext_handshake", len(msg))
	if msg[0] != 0 {
		return nil, fmt.Errorf("protocol error: should be ext handshake not %d", msg[0])
	}
	ext, _, err := bencode2.BDecodeDict(msg[1:])
	if err != nil {
		logrus.Debugf("failed to decode ext metadata")
		return nil, errors.WithStack(err)
	}
	metadataSize, ok1 := bencode2.GetInt(ext, "metadata_size")
	utMetadata, ok2 := bencode2.GetInt(ext, "m.ut_metadata")
	if !ok1 || !ok2 || metadataSize > maxMetadataSize {
		return nil, fmt.Errorf("protocol error: illegal format")
	}
	return &ExtHandshakeResult{
		MetadataSize: metadataSize,
		UtMetadata:   utMetadata,
		NumPieces:    int(math.Ceil(float64(metadataSize) / float64(pieceLength))),
	}, nil
}

func (bt *BitTorrent) requestPieces(ext *ExtHandshakeResult) ([]*Piece, error) {
	pieces := make([]*Piece, 0)
	for i := 0; i < ext.NumPieces; i++ {
		err := bt.requestPiece(ext, i)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}
	for i := 0; i < ext.NumPieces; i++ {
		piece, err := bt.readPiece()
		if err != nil {
			return nil, errors.WithStack(err)
		}
		pieces = append(pieces, piece)
	}
	return pieces, nil
}

func (bt *BitTorrent) requestPiece(ext *ExtHandshakeResult, piece int) error {
	p, err := bencode2.BEncode(map[string]any{
		"msg_type": 0,
		"piece":    piece,
	})
	if err != nil {
		return errors.WithStack(err)
	}
	msg := append([]byte{btMsgID, byte(ext.UtMetadata)}, p...)
	err = bt.sendMessage(msg)
	if err != nil {
		return errors.WithStack(err)
	}
	bt.trafficMetric("out_bt_request_piece", len(msg))
	return nil
}

func (bt *BitTorrent) readPiece() (*Piece, error) {
	msg, err := bt.readExtMessage()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	bt.trafficMetric("in_bt_piece", len(msg))
	if msg[0] == 0 {
		return nil, fmt.Errorf("protocol error: should not be ext handshake")
	}
	dict, pos, err := bencode2.BDecodeDict(msg[1:])
	if err != nil {
		logrus.Debugf("failed to decode piece")
		return nil, errors.WithStack(err)
	}
	trailer := msg[pos+1:]
	msgType, _ := bencode2.GetInt(dict, "msg_type")
	if msgType != 1 {
		return nil, fmt.Errorf("protocol error: wrong msg_type")
	}
	if len(trailer) > pieceLength {
		return nil, fmt.Errorf("protocol error: piece too long")
	}
	id, _ := bencode2.GetInt(dict, "piece")
	return &Piece{
		Data: trailer,
		ID:   id,
	}, nil
}

func (bt *BitTorrent) readMessage() ([]byte, error) {
	msgLengthB := make([]byte, 4)
	var msgLength uint32
	_, err := bt.read(msgLengthB)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	bt.trafficMetric("in_bt_message_header", len(msgLengthB))
	msgLength = binary.BigEndian.Uint32(msgLengthB)
	msg := make([]byte, msgLength)
	_, err = bt.read(msg)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return msg, nil
}

func (bt *BitTorrent) readExtMessage() ([]byte, error) {
	for {
		msg, err := bt.readMessage()
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if len(msg) == 0 {
			continue
		}
		bt.trafficMetric("in_bt_ext_message_header", 1)
		if msg[0] == btMsgID {
			return msg[1:], nil
		}
	}
}
