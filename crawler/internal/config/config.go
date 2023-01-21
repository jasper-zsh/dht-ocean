package config

import (
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	Ocean          zrpc.RpcClientConf
	Redis          redis.RedisConf
	DHTListen      string `json:",default=:6881"`
	BootstrapNodes []string
	MaxQueueSize   int    `json:",default=3000"`
	NodeID         string `json:",optional"`
}
