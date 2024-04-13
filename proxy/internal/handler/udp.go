package handler

import (
	"bytes"
	"context"
	"dht-ocean/proxy/internal/protocol"
	"net"

	"github.com/zeromicro/go-zero/core/logx"
)

var _ Handler = (*UDPHandler)(nil)

type UDPHandler struct {
	clientConn net.Conn
	localConn  *net.UDPConn
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewUDPHandler(ctx context.Context, conn net.Conn) (ret *UDPHandler) {
	ret = &UDPHandler{
		clientConn: conn,
	}
	ret.ctx, ret.cancel = context.WithCancel(ctx)
	return
}

func (u *UDPHandler) Run() {
	var err error
	u.localConn, err = net.ListenUDP("udp", nil)
	if err != nil {
		logx.Errorf("Failed to run udp handler: %+v", err)
		u.close()
		return
	}
	go u.send()
	go u.receive()
	<-u.ctx.Done()
	u.close()
}

func (u *UDPHandler) close() {
	if u.localConn != nil {
		u.localConn.Close()
		u.localConn = nil
	}
	if u.clientConn != nil {
		u.clientConn.Close()
		u.clientConn = nil
	}
}

func (u *UDPHandler) send() {
	hdr := protocol.UDPHeader{}
	var n int
	for {
		addrPort, err := hdr.ReadFrom(u.clientConn)
		if err != nil {
			logx.Errorf("Failed to read header from client: %+v", err)
			u.cancel()
			return
		}
		buf := make([]byte, hdr.Length)
		n, err = u.clientConn.Read(buf)
		if err != nil {
			logx.Errorf("Failed to read data from client: %+v", err)
			u.cancel()
			return
		}
		if n != int(hdr.Length) {
			logx.Errorf("Illegal data length, expected %d actual %d", hdr.Length, n)
			u.cancel()
			return
		}
		_, err = u.localConn.WriteToUDPAddrPort(buf[:n], addrPort)
		if err != nil {
			logx.Errorf("Failed to write to udp: %+v", err)
			u.cancel()
			return
		}
	}
}

func (u *UDPHandler) receive() {
	hdr := protocol.UDPHeader{}
	readBuf := make([]byte, 4096)
	writeBuf := make([]byte, 0, 4096)
	writer := bytes.NewBuffer(writeBuf)
	for {
		n, addrPort, err := u.localConn.ReadFromUDPAddrPort(readBuf)
		if err != nil {
			logx.Errorf("Failed to read data from udp: %+v", err)
			u.cancel()
			return
		}
		if n == len(readBuf) {
			logx.Errorf("packet size %d too large, expand buffer size and drop", n)
			readBuf = make([]byte, 2*len(readBuf))
			continue
		}
		hdr.Length = uint32(n)
		rawAddr, err := hdr.SetAddr(addrPort)
		if err != nil {
			logx.Errorf("Failed to set addr: %+v", err)
			continue
		}
		err = hdr.WriteTo(writer, rawAddr)
		if err != nil {
			logx.Errorf("Failed to write header to client: %+v", err)
			u.cancel()
			return
		}
		_, err = writer.Write(readBuf[:n])
		if err != nil {
			logx.Errorf("Failed to write to buffer: %+v", err)
			u.cancel()
			return
		}
		_, err = u.clientConn.Write(writer.Bytes())
		if err != nil {
			logx.Errorf("Failed to write data to client: %+v", err)
			u.cancel()
			return
		}
		writer.Reset()
	}
}
