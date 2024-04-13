package protocol

import (
	"encoding/binary"
	"io"
	"net/netip"

	"github.com/juju/errors"
)

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
	_, err = writer.Write(addr)
	if err != nil {
		return errors.Trace(err)
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
