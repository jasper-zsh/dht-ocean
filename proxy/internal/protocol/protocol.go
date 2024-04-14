package protocol

import (
	"encoding/binary"
	"io"
	"net/netip"

	"github.com/juju/errors"
)

const (
	ModeUDP = 0x01
)

type Handshake struct {
	Version byte
	Mode    byte
}

func (h *Handshake) ReadFrom(reader io.Reader) error {
	err := binary.Read(reader, binary.BigEndian, h)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (h *Handshake) WriteTo(writer io.Writer) error {
	err := binary.Write(writer, binary.BigEndian, h)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

const (
	AddrTypeIPv4 = 0x01
	AddrTypeIPv6 = 0x02
)

type AddrHeader struct {
	AddrType byte
	AddrLen  uint8
	Port     uint16
}

func (h *AddrHeader) SetAddr(addr netip.AddrPort) ([]byte, error) {
	if addr.Addr().Is6() {
		h.AddrType = AddrTypeIPv6
		h.Port = addr.Port()
		h.AddrLen = 16
	} else {
		h.AddrType = AddrTypeIPv4
		h.Port = addr.Port()
		h.AddrLen = 4
	}
	ip := addr.Addr().AsSlice()
	h.AddrLen = uint8(len(ip))
	return ip, nil
}

func (h *AddrHeader) ReadAddrPort(reader io.Reader) (addrPort netip.AddrPort, err error) {
	switch h.AddrType {
	case AddrTypeIPv4, AddrTypeIPv6:
	default:
		err = errors.New("unsupported addr type")
		return
	}
	rawAddr := make([]byte, h.AddrLen)
	var n int
	n, err = io.ReadFull(reader, rawAddr)
	if err != nil {
		err = errors.Trace(err)
		return
	}
	if n != int(h.AddrLen) {
		err = errors.New("invalid addr len")
		return
	}
	addr, ok := netip.AddrFromSlice(rawAddr)
	if !ok {
		err = errors.New("illegal addr")
		return
	}
	addrPort = netip.AddrPortFrom(addr, h.Port)
	return
}
