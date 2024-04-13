package internal

import (
	"dht-ocean/proxy/internal/handler"
	"dht-ocean/proxy/internal/protocol"
	"net"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProxyServerOptions struct {
	Listen string
}

type ProxyServer struct {
	options ProxyServerOptions

	listener net.Listener
	handlers map[string]handler.Handler
}

func NewProxyServer(options ProxyServerOptions) *ProxyServer {
	return &ProxyServer{
		options:  options,
		handlers: make(map[string]handler.Handler),
	}
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
	if s.listener != nil {
		s.listener.Close()
	}
	for _, handler := range s.handlers {
		handler.Stop()
	}
	s.handlers = make(map[string]handler.Handler)
}

// FIXME: leak when closed by client side
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
			handler := handler.NewUDPHandler(conn)
			err := handler.Start()
			if err != nil {
				logx.Errorf("Failed to create udp handler: %+v", err)
				conn.Close()
				continue
			}
			key := conn.RemoteAddr().String()
			h, ok := s.handlers[key]
			if ok {
				h.Stop()
			}
			s.handlers[key] = handler
			logx.Infof("Created UDP Handler: %s", key)
		}
	}
}
