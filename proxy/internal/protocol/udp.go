package protocol

import (
	"encoding/binary"
	"io"
	"net/netip"

	"github.com/juju/errors"
)

var UDPHeaderSize = binary.Size(UDPHeader{})

type UDPHeader struct {
	Version byte
	AddrHeader
	Length uint32
}

func (h *UDPHeader) WriteTo(writer io.Writer, addr []byte) error {
	err := binary.Write(writer, binary.BigEndian, h)
	if err != nil {
		return errors.Trace(err)
	}
	n, err := writer.Write(addr)
	if err != nil {
		return errors.Trace(err)
	}
	if n != len(addr) {
		return io.ErrShortWrite
	}
	return nil
}

func (h *UDPHeader) ReadFrom(reader io.Reader) (addrPort netip.AddrPort, err error) {
	err = binary.Read(reader, binary.BigEndian, h)
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
