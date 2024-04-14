package client

import (
	"bytes"
	"dht-ocean/proxy/internal/protocol"
	"io"
	"net"
	"net/netip"
	"time"

	"github.com/juju/errors"
)

var _ net.PacketConn = (*UDPConn)(nil)

type UDPConn struct {
	proxyConn net.Conn

	readHeader protocol.UDPHeader

	writeBuf    *bytes.Buffer
	writeHeader protocol.UDPHeader
}

func NewUDPConn(conn net.Conn) *UDPConn {
	return &UDPConn{
		proxyConn: conn,
		writeBuf:  bytes.NewBuffer(make([]byte, 0, 4096)),
	}
}

// ReadFrom implements net.PacketConn.
func (u *UDPConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	var addrPort netip.AddrPort
	addrPort, err = u.readHeader.ReadFrom(u.proxyConn)
	if err != nil {
		err = errors.Trace(err)
		return
	}
	addr = net.UDPAddrFromAddrPort(addrPort)

	n, err = io.ReadFull(u.proxyConn, p[:u.readHeader.Length])
	if err != nil {
		err = errors.Trace(err)
		return
	}
	return
}

// WriteTo implements net.PacketConn.
func (u *UDPConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	u.writeHeader.Length = uint32(len(p))
	var addrPort netip.AddrPort
	addrPort, err = netip.ParseAddrPort(addr.String())
	if err != nil {
		err = errors.Trace(err)
		return
	}
	var rawAddr []byte
	rawAddr, err = u.writeHeader.SetAddr(addrPort)
	if err != nil {
		err = errors.Trace(err)
		return
	}
	err = u.writeHeader.WriteTo(u.writeBuf, rawAddr)
	if err != nil {
		err = errors.Trace(err)
		return
	}
	n, err = u.writeBuf.Write(p)
	if err != nil {
		err = errors.Trace(err)
		return
	}
	_, err = u.proxyConn.Write(u.writeBuf.Bytes())
	if err != nil {
		err = errors.Trace(err)
		return
	}
	u.writeBuf.Reset()
	return
}

// Close implements net.PacketConn.
func (u *UDPConn) Close() error {
	if u.proxyConn != nil {
		u.proxyConn.Close()
	}
	return nil
}

// LocalAddr implements net.PacketConn.
func (u *UDPConn) LocalAddr() net.Addr {
	return nil
}

// SetDeadline implements net.PacketConn.
func (u *UDPConn) SetDeadline(t time.Time) error {
	return errors.Trace(u.SetDeadline(t))
}

// SetReadDeadline implements net.PacketConn.
func (u *UDPConn) SetReadDeadline(t time.Time) error {
	return errors.Trace(u.proxyConn.SetReadDeadline(t))
}

// SetWriteDeadline implements net.PacketConn.
func (u *UDPConn) SetWriteDeadline(t time.Time) error {
	return errors.Trace(u.proxyConn.SetWriteDeadline(t))
}
