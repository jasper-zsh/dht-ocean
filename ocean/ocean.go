package main

import (
	"context"
	"dht-ocean/ocean/internal/model"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/kamva/mgm/v3"
	"github.com/zeromicro/go-zero/core/bloom"
	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"dht-ocean/ocean/internal/config"
	"dht-ocean/ocean/internal/server"
	"dht-ocean/ocean/internal/svc"
	"dht-ocean/ocean/ocean"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/ocean.yaml", "the config file")
var bloomFlag = flag.Bool("bloom", false, "Build bloom filter")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	svcCtx := svc.NewServiceContext(c)

	if *bloomFlag {
		ctx := context.TODO()
		redis := c.Redis.NewRedis()
		filter := bloom.New(redis, "torrent_bloom", 1024*1024*5)
		coll := mgm.Coll(&model.Torrent{})
		opts := options.Find().SetProjection(bson.M{
			"_id": true,
		})
		cursor, err := coll.Find(ctx, bson.D{}, opts)
		if err != nil {
			panic(err)
		}
		cnt := 0
		for cursor.Next(ctx) {
			idStr := cursor.Current.Index(0).Value().StringValue()
			idBytes, err := hex.DecodeString(idStr)
			if err != nil {
				logx.Errorf("Failed to convert info hash %s to bytes. %v", idStr, err)
				continue
			}
			err = filter.Add(idBytes)
			if err != nil {
				logx.Errorf("Failed to add info hash %s to bloom filter. %v", idStr, err)
				continue
			}
			cnt += 1
			if cnt%100 == 0 {
				logx.Infof("Added %d info hashes", cnt)
			}
		}

		return
	}

	group := service.NewServiceGroup()

	group.Add(zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		ocean.RegisterOceanServer(grpcServer, server.NewOceanServer(svcCtx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	}))
	group.Add(svcCtx.Indexer)

	stats := svc.NewStats(svcCtx)
	group.Add(stats)
	defer group.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	group.Start()
}
