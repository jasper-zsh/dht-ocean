package main

import (
	"context"
	"dht-ocean/crawler/internal/config"
	"dht-ocean/crawler/internal/svc"
	"flag"
	_ "net/http/pprof"
	"os"
	"os/signal"

	"github.com/sirupsen/logrus"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
)

var configFile = flag.String("f", "etc/crawler.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	c.MustSetUp()

	ctx, _ := signal.NotifyContext(context.TODO(), os.Interrupt, os.Kill)
	svcCtx := svc.NewServiceContext(c, ctx)

	group := service.NewServiceGroup()
	group.Add(svcCtx.Crawler)
	group.Add(svcCtx.TorrentFetcher)
	group.Add(svcCtx.DBPersist)
	defer group.Stop()

	logrus.Infof("Starting crawler...")
	group.Start()
}
