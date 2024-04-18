package main

import (
	"context"
	"dht-ocean/common/model"
	"dht-ocean/indexer/internal/config"
	"dht-ocean/indexer/internal/svc"
	"flag"
	"os"
	"os/signal"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
)

var configFile = flag.String("f", "etc/indexer.yaml", "the config file")

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

	ctx, _ := signal.NotifyContext(context.TODO(), os.Interrupt, os.Kill)
	indexer, err := svc.NewIndexer(ctx, &c)
	if err != nil {
		logx.Errorf("Failed to initialize indexer: %+v", err)
		panic(err)
	}

	group := service.NewServiceGroup()
	defer group.Stop()

	group.Add(indexer)

	logx.Info("Starting indexer...")
	group.Start()
}
