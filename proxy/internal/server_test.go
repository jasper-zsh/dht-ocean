package internal

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var proxyServer *ProxyServer
var proxyClient *ProxyClient

const (
	serverAddr = "127.0.0.1:54321"
)

func TestMain(m *testing.M) {
	proxyServer = NewProxyServer(ProxyServerOptions{
		Listen: serverAddr,
	})
	go proxyServer.Start()
	time.Sleep(100 * time.Millisecond)

	proxyClient = NewProxyClient(ProxyClientOptions{
		Server: serverAddr,
	})
	m.Run()
	proxyServer.Stop()
}

func TestBasic(t *testing.T) {
	s, err := net.ListenUDP("udp", net.UDPAddrFromAddrPort(netip.MustParseAddrPort("127.0.0.1:54322")))
	if !assert.NoError(t, err) {
		return
	}
	c, err := proxyClient.ListenUDP()
	if !assert.NoError(t, err) {
		return
	}
	if !assert.NotNil(t, c) {
		return
	}

	// Test client to server
	data := []byte("foo")
	n, err := c.WriteTo(data, s.LocalAddr())
	if !assert.NoError(t, err) {
		return
	}
	if !assert.Equal(t, len(data), n) {
		return
	}

	buf := make([]byte, 4096)
	n, addr, err := s.ReadFromUDP(buf)
	if !assert.NoError(t, err) {
		return
	}
	if !assert.Equal(t, data, buf[:n]) {
		return
	}

	// Test server to client
	_, err = s.WriteToUDP(data, addr)
	if !assert.NoError(t, err) {
		return
	}
	n, _, err = c.ReadFrom(buf)
	if !assert.NoError(t, err) {
		return
	}
	if !assert.Equal(t, data, buf[:n]) {
		return
	}
}
