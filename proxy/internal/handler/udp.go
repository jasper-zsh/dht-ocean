package handler

import (
	"dht-ocean/proxy/internal/protocol"
	"net"

	"github.com/juju/errors"
	"github.com/zeromicro/go-zero/core/logx"
)

var _ Handler = (*UDPHandler)(nil)

type UDPHandler struct {
	clientConn net.Conn
	localConn  *net.UDPConn
}

func NewUDPHandler(conn net.Conn) (ret *UDPHandler) {
	ret = &UDPHandler{
		clientConn: conn,
	}
	return
}

// Start implements Handler.
func (u *UDPHandler) Start() (err error) {
	if u.localConn == nil {
		u.localConn, err = net.ListenUDP("udp", nil)
		if err != nil {
			err = errors.Trace(err)
			return
		}
		go u.send()
		go u.receive()
	}
	return
}

// Stop implements Handler.
func (u *UDPHandler) Stop() {
	if u.clientConn != nil {
		u.clientConn.Close()
		u.clientConn = nil
	}
	if u.localConn != nil {
		u.localConn.Close()
		u.localConn = nil
	}
}

func (u *UDPHandler) send() {
	hdr := protocol.UDPHeader{}
	buf := make([]byte, 4096)
	var n int
	for {
		addrPort, err := hdr.ReadFrom(u.clientConn)
		if err != nil {
			logx.Errorf("Failed to read header from client: %+v", err)
			u.Stop()
			return
		}
		n, err = u.clientConn.Read(buf)
		if err != nil {
			logx.Errorf("Failed to read data from client: %+v", err)
			u.Stop()
			return
		}
		if n != int(hdr.Length) {
			logx.Errorf("Illegal data length, expected %d actual %d", hdr.Length, n)
			u.Stop()
			return
		}
		_, err = u.localConn.WriteToUDPAddrPort(buf[:n], addrPort)
		if err != nil {
			logx.Errorf("Failed to write to udp: %+v", err)
			u.Stop()
			return
		}
	}
}

func (u *UDPHandler) receive() {
	hdr := protocol.UDPHeader{}
	buf := make([]byte, 4096)
	for {
		n, addrPort, err := u.localConn.ReadFromUDPAddrPort(buf)
		if err != nil {
			logx.Errorf("Failed to read data from udp: %+v", err)
			u.Stop()
			return
		}
		hdr.Length = uint32(n)
		rawAddr, err := hdr.SetAddr(addrPort)
		if err != nil {
			logx.Errorf("Failed to set addr: %+v", err)
			continue
		}
		err = hdr.WriteTo(u.clientConn, rawAddr)
		if err != nil {
			logx.Errorf("Failed to write header to client: %+v", err)
			u.Stop()
			return
		}
		_, err = u.clientConn.Write(buf[:n])
		if err != nil {
			logx.Errorf("Failed to write data to client: %+v", err)
			u.Stop()
			return
		}
	}
}
