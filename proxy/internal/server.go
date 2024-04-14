package internal

import (
	"context"
	"dht-ocean/proxy/internal/handler"
	"dht-ocean/proxy/internal/protocol"
	"net"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProxyServerOptions struct {
	Listen     string
	BufferSize int
}

type ProxyServer struct {
	options ProxyServerOptions

	listener net.Listener
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewProxyServer(options ProxyServerOptions) *ProxyServer {
	s := &ProxyServer{
		options: options,
	}
	s.ctx, s.cancel = context.WithCancel(context.TODO())
	return s
}

func (s *ProxyServer) Start() {
	if s.listener != nil {
		return
	}
	var err error
	s.listener, err = net.Listen("tcp", s.options.Listen)
	if err != nil {
		panic(err)
	}
	logx.Infof("Listen %s", s.options.Listen)
	s.listen()
}

func (s *ProxyServer) Stop() {
	s.cancel()
	if s.listener != nil {
		s.listener.Close()
	}
}

func (s *ProxyServer) listen() {
	handshake := protocol.Handshake{}
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			logx.Errorf("Failed to accept proxy connection: %+v", err)
			return
		}
		err = handshake.ReadFrom(conn)
		if err != nil {
			logx.Errorf("Handshake failed: %+v", err)
			continue
		}
		switch handshake.Mode {
		case protocol.ModeUDP:
			handler := handler.NewUDPHandler(s.ctx, conn, s.options.BufferSize)
			go handler.Run()
			logx.Infof("Created UDP Handler: %s", conn.RemoteAddr().String())
		}
	}
}
