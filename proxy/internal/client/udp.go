package client

import (
	"bufio"
	"dht-ocean/proxy/internal/protocol"
	"encoding/binary"
	"io"
	"net"
	"net/netip"
	"time"

	"github.com/juju/errors"
	"github.com/zeromicro/go-zero/core/logx"
)

var _ net.PacketConn = (*UDPConn)(nil)

type UDPConn struct {
	proxyConn net.Conn
	reader    *bufio.Reader
	writer    *bufio.Writer

	readHeader  protocol.UDPHeader
	writeHeader protocol.UDPHeader
}

func NewUDPConn(conn net.Conn, lport uint16, bufSize int) (*UDPConn, error) {
	ret := &UDPConn{
		proxyConn: conn,
		reader:    bufio.NewReaderSize(conn, bufSize),
		writer:    bufio.NewWriterSize(conn, bufSize),
	}
	handshake := protocol.UDPHandshake{
		Port: lport,
	}
	err := binary.Write(conn, binary.BigEndian, &handshake)
	if err != nil {
		conn.Close()
		return nil, errors.Trace(err)
	}
	return ret, nil
}

// ReadFrom implements net.PacketConn.
func (u *UDPConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	var addrPort netip.AddrPort
	addrPort, err = u.readHeader.ReadFrom(u.reader)
	if err != nil {
		err = errors.Trace(err)
		return
	}
	addr = net.UDPAddrFromAddrPort(addrPort)

	if u.reader.Size() < int(u.readHeader.Length) {
		newSize := u.reader.Size() * 2
		logx.Infof("reader size not big enough, expand to %d", newSize)
		u.reader = bufio.NewReaderSize(u.reader, newSize)
	}
	n, err = io.ReadFull(u.reader, p[:u.readHeader.Length])
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
	err = u.writeHeader.WriteTo(u.writer, rawAddr)
	if err != nil {
		err = errors.Trace(err)
		return
	}
	n, err = u.writer.Write(p)
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
