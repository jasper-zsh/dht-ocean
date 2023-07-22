package main

import (
	config2 "dht-ocean/crawler/internal/config"
	"dht-ocean/crawler/internal/svc"
	"flag"
	_ "net/http/pprof"

	"github.com/sirupsen/logrus"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
)

var configFile = flag.String("f", "etc/crawler.yaml", "the config file")

func main() {
	flag.Parse()

	var c config2.Config
	conf.MustLoad(*configFile, &c)
	c.MustSetUp()
	ctx := svc.NewServiceContext(c)

	group := service.NewServiceGroup()
	group.Add(ctx.Crawler)
	group.Add(ctx.TorrentFetcher)
	defer group.Stop()

	logrus.Infof("Starting crawler...")
	group.Start()
}
