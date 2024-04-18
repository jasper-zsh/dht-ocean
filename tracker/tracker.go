package main

import (
	"context"
	"dht-ocean/common/model"
	config "dht-ocean/tracker/internal/config"
	"dht-ocean/tracker/internal/svc"
	"flag"
	"os"
	"os/signal"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
)

var configFile = flag.String("f", "etc/tracker.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	c.MustSetUp()

	err := model.InitMongo("dht_ocean", c.Mongo)
	if err != nil {
		logx.Errorf("Failed to initialize MongoDB: %+v", err)
		panic(err)
	}

	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	svcCtx := svc.NewServiceContext(ctx, c)
	err = svcCtx.Tracker.Start()
	if err != nil {
		panic(err)
	}

	group := service.NewServiceGroup()
	group.Add(svcCtx.Updater)
	defer group.Stop()

	logx.Infof("Starting tracker...")
	group.Start()
}
