package proxy

import (
	"dht-ocean/proxy/internal/client"
	"dht-ocean/proxy/internal/protocol"
	"net"
	"net/netip"

	"github.com/juju/errors"
)

type ProxyClientOptions struct {
	Server  string
	BufSize int
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

func (c *ProxyClient) ListenUDP(lport uint16) (net.PacketConn, error) {
	addr, err := netip.ParseAddrPort(c.options.Server)
	if err != nil {
		return nil, errors.Trace(err)
	}
	conn, err := net.DialTCP("tcp", nil, net.TCPAddrFromAddrPort(addr))
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
	udpConn, err := client.NewUDPConn(conn, lport, c.options.BufSize)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return udpConn, nil
}
