package bittorrent

import (
	"bytes"
	"crypto/sha1"
	"dht-ocean/bencode"
	"dht-ocean/dht"
	"encoding/binary"
	"fmt"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"io"
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
	nodeID   []byte
}

func NewBitTorrent(nodeID, infoHash []byte, addr string) *BitTorrent {
	r := &BitTorrent{
		addr:     addr,
		infoHash: infoHash,
		nodeID:   nodeID,
	}
	return r
}

func (bt *BitTorrent) Start() error {
	conn, err := net.Dial("tcp", bt.addr)
	if err != nil {
		return errors.WithStack(err)
	}
	bt.conn = conn
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
	_, err := bt.conn.Write(lenB)
	if err != nil {
		return err
	}
	_, err = bt.conn.Write(msg)
	if err != nil {
		return err
	}
	return nil
}

func (bt *BitTorrent) GetTorrent() (*Torrent, error) {
	err := bt.handshake()
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
	if !bytes.Equal(bt.infoHash, hash) {
		return nil, fmt.Errorf("corrupt torrent")
	}
	metadata, _, err := bencode.BDecodeDict(rawMetadata)
	if err != nil {
		return nil, err
	}
	return NewTorrentFromMetadata(bt.infoHash, metadata), nil
}

func (bt *BitTorrent) handshake() error {
	pkt := make([]byte, 1)
	pkt[0] = byte(len(btProtocol))
	pkt = append(pkt, btProtocol...)
	pkt = append(pkt, btReserved...)
	pkt = append(pkt, bt.infoHash...)
	// AlphaReign
	pkt = append(pkt, dht.GenerateNodeID()...)
	// Official
	//pkt = append(pkt, bt.nodeID...)
	_, err := bt.conn.Write(pkt)
	if err != nil {
		return errors.WithStack(err)
	}
	pLength := make([]byte, 1)
	b, err := io.ReadFull(bt.conn, pLength)
	if err != nil {
		return errors.WithStack(err)
	}
	if b == 0 {
		return fmt.Errorf("read empty protocol length")
	}
	dataLen := int(pLength[0]) + 48
	data := make([]byte, dataLen)
	_, err = io.ReadFull(bt.conn, data)
	if err != nil {
		return errors.WithStack(err)
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
		return nil, errors.WithStack(err)
	}
	msg := append([]byte{btMsgID, extHandshakeID}, []byte(p)...)
	err = bt.sendMessage(msg)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	msg, err = bt.readExtMessage()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if msg[0] != 0 {
		return nil, fmt.Errorf("protocol error: should be ext handshake not %d", msg[0])
	}
	ext, _, err := bencode.BDecodeDict(msg[1:])
	if err != nil {
		logrus.Debugf("failed to decode ext metadata")
		return nil, errors.WithStack(err)
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
	p, err := bencode.BEncode(map[string]any{
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
	return nil
}

func (bt *BitTorrent) readPiece() (*Piece, error) {
	msg, err := bt.readExtMessage()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if msg[0] == 0 {
		return nil, fmt.Errorf("protocol error: should not be ext handshake")
	}
	dict, pos, err := bencode.BDecodeDict(msg[1:])
	if err != nil {
		logrus.Debugf("failed to decode piece")
		return nil, errors.WithStack(err)
	}
	trailer := msg[pos+1:]
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
	var msgLength uint32
	_, err := io.ReadFull(bt.conn, msgLengthB)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	msgLength = binary.BigEndian.Uint32(msgLengthB)
	msg := make([]byte, msgLength)
	_, err = io.ReadFull(bt.conn, msg)
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
		if msg[0] == btMsgID {
			return msg[1:], nil
		}
	}
}
