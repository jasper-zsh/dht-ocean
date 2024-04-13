package client

import (
	"bytes"
	"dht-ocean/proxy/internal/protocol"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/juju/errors"
)

var _ net.PacketConn = (*UDPConn)(nil)

type UDPConn struct {
	proxyConn net.Conn

	readHeader protocol.UDPHeader
	readLock   sync.Mutex

	writeBuf    []byte
	writeHeader protocol.UDPHeader
	writeLock   sync.Mutex
}

func NewUDPConn(conn net.Conn) *UDPConn {
	return &UDPConn{
		proxyConn: conn,
	}
}

// ReadFrom implements net.PacketConn.
func (u *UDPConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	// u.readLock.Lock()
	// defer u.readLock.Unlock()

	var addrPort netip.AddrPort
	addrPort, err = u.readHeader.ReadFrom(u.proxyConn)
	if err != nil {
		err = errors.Trace(err)
		return
	}
	addr = net.UDPAddrFromAddrPort(addrPort)

	buf := make([]byte, u.readHeader.Length)
	n, err = u.proxyConn.Read(buf)
	if err != nil {
		err = errors.Trace(err)
		return
	}
	copy(p[:n], buf[:n])
	return
}

// WriteTo implements net.PacketConn.
func (u *UDPConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	// u.writeLock.Lock()
	// defer u.writeLock.Unlock()

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
	writer := bytes.NewBuffer(u.writeBuf)
	err = u.writeHeader.WriteTo(writer, rawAddr)
	if err != nil {
		err = errors.Trace(err)
		return
	}
	n, err = writer.Write(p)
	if err != nil {
		err = errors.Trace(err)
		return
	}
	_, err = u.proxyConn.Write(writer.Bytes())
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
