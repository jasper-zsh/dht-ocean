package bittorrent

import (
	"bytes"
	"dht-ocean/bencode"
	"dht-ocean/dht/protocol"
	"encoding/binary"
	"fmt"
	"github.com/sirupsen/logrus"
	"math"
	"net"
)

var (
	btProtocol           = []byte("BitTorrent protocol")
	btReserved           = []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x10, 0x00, 0x01}
	pieceLength          = int(math.Pow(2, 14))
	maxMetadataSize      = 10000000
	extHandshakeID  byte = 0
	btMsgID         byte = 20
)

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
	addr     string
	conn     net.Conn
	infoHash []byte
}

func NewBitTorrent(infoHash []byte, addr string) *BitTorrent {
	r := &BitTorrent{
		addr:     addr,
		infoHash: infoHash,
	}
	return r
}

func (bt *BitTorrent) Start() error {
	conn, err := net.Dial("tcp", bt.addr)
	if err != nil {
		return err
	}
	bt.conn = conn
	return nil
}

func (bt *BitTorrent) Stop() error {
	err := bt.conn.Close()
	if err != nil {
		return err
	}
	return nil
}

func (bt *BitTorrent) GetMetadata() error {
	err := bt.handshake()
	if err != nil {
		return err
	}
	ext, err := bt.extHandshake()
	if err != nil {
		return err
	}
	pieces, err := bt.requestPieces(ext)
	if err != nil {
		return err
	}
	logrus.Infof("got pieces: %s", pieces)
	return nil
}

func (bt *BitTorrent) handshake() error {
	pkt := append([]byte{(byte)(len(btProtocol))}, btProtocol...)
	pkt = append(pkt, btReserved...)
	pkt = append(pkt, bt.infoHash...)
	pkt = append(pkt, protocol.GenerateNodeID()...)
	_, err := bt.conn.Write(pkt)
	if err != nil {
		return err
	}
	pLength := make([]byte, 1)
	b, err := bt.conn.Read(pLength)
	if err != nil {
		return err
	}
	if b == 0 {
		return fmt.Errorf("read empty protocol length")
	}
	data := make([]byte, pLength[0]+48)
	_, err = bt.conn.Read(data)
	if err != nil {
		return err
	}
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
	p, err := bencode.BEncode(map[string]any{
		"m": map[string]any{
			"ut_metadata": 1,
		},
	})
	if err != nil {
		return nil, err
	}
	msg := append([]byte{btMsgID, extHandshakeID}, []byte(p)...)
	_, err = bt.conn.Write(msg)
	if err != nil {
		return nil, err
	}
	msg, err = bt.readExtMessage()
	if msg[0] != 0 {
		return nil, fmt.Errorf("protocol error: should be ext handshake not %d", msg[0])
	}
	ext, _, err := bencode.BDecodeDict([]byte(p))
	if err != nil {
		return nil, err
	}
	metadataSize, ok1 := bencode.GetInt(ext, "metadata_size")
	utMetadata, ok2 := bencode.GetInt(ext, "m.ut_metadata")
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
	pieces := make([]*Piece, 0, ext.NumPieces)
	for i := 0; i < ext.NumPieces; i++ {
		err := bt.requestPiece(ext, i)
		if err != nil {
			return nil, err
		}
		piece, err := bt.readPiece()
		if err != nil {
			return nil, err
		}
		pieces = append(pieces, piece)
	}
	return pieces, nil
}

func (bt *BitTorrent) requestPiece(ext *ExtHandshakeResult, piece int) error {
	p, err := bencode.BEncode(map[string]any{
		"msg_type": 0,
		"piece":    piece,
	})
	if err != nil {
		return err
	}
	msg := append([]byte{btMsgID, byte(ext.UtMetadata)}, p...)
	_, err = bt.conn.Write(msg)
	if err != nil {
		return err
	}
	return nil
}

func (bt *BitTorrent) readPiece() (*Piece, error) {
	msg, err := bt.readExtMessage()
	if err != nil {
		return nil, err
	}
	if msg[0] == 0 {
		return nil, fmt.Errorf("protocol error: should not be ext handshake")
	}
	dict, pos, err := bencode.BDecodeDict(msg[1:])
	if err != nil {
		return nil, err
	}
	trailer := msg[pos:]
	msgType, _ := bencode.GetInt(dict, "msg_type")
	if msgType != 1 {
		return nil, fmt.Errorf("protocol error: wrong msg_type")
	}
	if len(trailer) > pieceLength {
		return nil, fmt.Errorf("protocol error: piece too long")
	}
	id, _ := bencode.GetInt(dict, "piece")
	return &Piece{
		Data: trailer,
		ID:   id,
	}, nil
}

func (bt *BitTorrent) readMessage() ([]byte, error) {
	msgLengthB := make([]byte, 4)
	_, err := bt.conn.Read(msgLengthB)
	if err != nil {
		return nil, err
	}
	msgLength := binary.BigEndian.Uint32(msgLengthB)
	if msgLength == 0 {
		return nil, nil
	}
	msg := make([]byte, msgLength)
	_, err = bt.conn.Read(msg)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (bt *BitTorrent) readExtMessage() ([]byte, error) {
	for {
		msg, err := bt.readMessage()
		if err != nil {
			return nil, err
		}
		if msg[0] == btMsgID {
			return msg[1:], nil
		}
	}
}
