package config

import (
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	Ocean          zrpc.RpcClientConf
	Redis          redis.RedisConf
	DHTListen      string
	BootstrapNodes []string
	MaxQueueSize   int
}
