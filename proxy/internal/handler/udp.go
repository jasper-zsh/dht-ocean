package handler

import (
	"bufio"
	"context"
	"dht-ocean/proxy/internal/protocol"
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"github.com/zeromicro/go-zero/core/logx"
)

var _ Handler = (*UDPHandler)(nil)

type UDPHandler struct {
	clientConn net.Conn
	localConn  *net.UDPConn

	buffered io.ReadWriter

	ctx    context.Context
	cancel context.CancelFunc
}

func NewUDPHandler(ctx context.Context, conn net.Conn, bufSize int) (ret *UDPHandler) {
	ret = &UDPHandler{
		clientConn: conn,
		buffered: bufio.NewReadWriter(
			bufio.NewReaderSize(conn, bufSize),
			bufio.NewWriterSize(conn, bufSize),
		),
	}
	ret.ctx, ret.cancel = context.WithCancel(ctx)
	return
}

func (u *UDPHandler) Run() {
	handshake := protocol.UDPHandshake{}
	err := binary.Read(u.clientConn, binary.BigEndian, &handshake)
	if err != nil {
		logx.Errorf("UDP handshake failed: %+v", err)
		u.close()
		return
	}
	var laddr *net.UDPAddr
	if handshake.Port > 0 {
		laddr, _ = net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", handshake.Port))
	}
	u.localConn, err = net.ListenUDP("udp", laddr)
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
	pipeBuf := make([]byte, 4096)
	for {
		addrPort, err := hdr.ReadFrom(u.buffered)
		if err != nil {
			logx.Errorf("Failed to read header from client: %+v", err)
			u.cancel()
			return
		}
		if int(hdr.Length) > len(pipeBuf) {
			logx.Infof("read buf size %d smaller than packet size %d, extend", len(pipeBuf), hdr.Length)
			pipeBuf = make([]byte, 2*len(pipeBuf))
		}
		n, err = io.ReadFull(u.buffered, pipeBuf[:hdr.Length])
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
		_, err = u.localConn.WriteToUDPAddrPort(pipeBuf[:n], addrPort)
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
		err = hdr.WriteTo(u.buffered, rawAddr)
		if err != nil {
			logx.Errorf("Failed to write header to client: %+v", err)
			u.cancel()
			return
		}
		_, err = u.buffered.Write(readBuf[:n])
		if err != nil {
			logx.Errorf("Failed to write data to client: %+v", err)
			u.cancel()
			return
		}
	}
}
