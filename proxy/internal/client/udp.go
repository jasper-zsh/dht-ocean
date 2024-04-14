package client

import (
	"bufio"
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
	buffered  io.ReadWriter

	readHeader  protocol.UDPHeader
	writeHeader protocol.UDPHeader
}

func NewUDPConn(conn net.Conn, bufSize int) *UDPConn {
	return &UDPConn{
		proxyConn: conn,
		buffered: bufio.NewReadWriter(
			bufio.NewReaderSize(conn, bufSize),
			bufio.NewWriterSize(conn, bufSize),
		),
	}
}

// ReadFrom implements net.PacketConn.
func (u *UDPConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	var addrPort netip.AddrPort
	addrPort, err = u.readHeader.ReadFrom(u.buffered)
	if err != nil {
		err = errors.Trace(err)
		return
	}
	addr = net.UDPAddrFromAddrPort(addrPort)

	n, err = io.ReadFull(u.buffered, p[:u.readHeader.Length])
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
	err = u.writeHeader.WriteTo(u.buffered, rawAddr)
	if err != nil {
		err = errors.Trace(err)
		return
	}
	n, err = u.buffered.Write(p)
	if err != nil {
		err = errors.Trace(err)
		return
	}
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
