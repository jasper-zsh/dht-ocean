package main

import (
	"dht-ocean/proxy/internal"
	"dht-ocean/proxy/internal/config"
	"flag"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
)

var configFile = flag.String("f", "etc/proxy.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	c.MustSetUp()

	g := service.NewServiceGroup()
	g.Add(internal.NewProxyServer(internal.ProxyServerOptions{
		Listen: c.Listen,
	}))
	defer g.Stop()

	logx.Info("Starting proxy...")
	g.Start()
}
