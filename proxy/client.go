package proxy

import (
	"dht-ocean/proxy/internal/client"
	"dht-ocean/proxy/internal/protocol"
	"net"

	"github.com/juju/errors"
)

type ProxyClientOptions struct {
	Server string
}

type ProxyClient struct {
	options ProxyClientOptions
}

func NewProxyClient(options ProxyClientOptions) (ret *ProxyClient) {
	ret = &ProxyClient{
		options: options,
	}
	return
}

func (c *ProxyClient) ListenUDP() (net.PacketConn, error) {
	conn, err := net.Dial("tcp", c.options.Server)
	if err != nil {
		return nil, errors.Trace(err)
	}
	handshake := protocol.Handshake{
		Mode: protocol.ModeUDP,
	}
	err = handshake.WriteTo(conn)
	if err != nil {
		conn.Close()
		return nil, errors.Trace(err)
	}
	return client.NewUDPConn(conn), nil
}
