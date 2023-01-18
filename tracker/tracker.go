package main

import (
	config "dht-ocean/tracker/internal/config"
	"dht-ocean/tracker/internal/svc"
	"flag"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
)

var configFile = flag.String("f", "etc/tracker.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	ctx := svc.NewServiceContext(c)

	group := service.NewServiceGroup()
	group.Add(ctx.Tracker)
	group.Add(ctx.Updater)
	defer group.Stop()

	logx.Infof("Starting tracker...")
	group.Start()
}
