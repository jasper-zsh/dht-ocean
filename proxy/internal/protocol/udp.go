package protocol

import (
	"bytes"
	"encoding/binary"
	"io"
	"net/netip"

	"github.com/juju/errors"
)

var UDPHeaderSize = binary.Size(UDPHeader{})

type UDPHeader struct {
	Version byte
	AddrHeader
	Length   uint32
	Checksum byte
}

func checksum(data []byte, length int) {
	data[length-1] = 0
	t := 0
	for _, v := range data {
		t += int(v)
	}
	data[length-1] = byte(t % 0xFF)
}

func (h *UDPHeader) WriteTo(writer io.Writer, addr []byte) error {
	h.Checksum = 0
	b := bytes.NewBuffer(make([]byte, 0, UDPHeaderSize))
	err := binary.Write(b, binary.BigEndian, h)
	if err != nil {
		return errors.Trace(err)
	}
	data := b.Bytes()
	checksum(data, len(data))
	n, err := writer.Write(data)
	if err != nil {
		return errors.Trace(err)
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	n, err = writer.Write(addr)
	if err != nil {
		return errors.Trace(err)
	}
	if n != len(addr) {
		return io.ErrShortWrite
	}
	return nil
}

func (h *UDPHeader) ReadFrom(reader io.Reader) (addrPort netip.AddrPort, err error) {
	raw := make([]byte, UDPHeaderSize)
	_, err = io.ReadFull(reader, raw)
	if err != nil {
		err = errors.Trace(err)
		return
	}
	c := raw[len(raw)-1]
	checksum(raw, len(raw))
	if c != raw[len(raw)-1] {
		err = errors.New("checksum not match")
		return
	}
	err = binary.Read(bytes.NewReader(raw), binary.BigEndian, h)
	if err != nil {
		err = errors.Trace(err)
		return
	}
	addrPort, err = h.ReadAddrPort(reader)
	if err != nil {
		err = errors.Trace(err)
		return
	}
	return
}
