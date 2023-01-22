package config

import (
	"github.com/zeromicro/go-zero/core/prometheus"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	Ocean               zrpc.RpcClientConf
	Redis               redis.RedisConf
	Prometheus          prometheus.Config `json:",optional"`
	DHTListen           string            `json:",default=:6881"`
	BootstrapNodes      []string
	FindNodeRateLimit   int    `json:",default=3000"`
	MaxQueueSize        int    `json:",default=3000"`
	NodeID              string `json:",optional"`
	TorrentWorkers      int    `json:",default=100"`
	TorrentMaxQueueSize int    `json:",default=1000"`
}

func (c *Config) SetUp() {
	prometheus.StartAgent(c.Prometheus)
}
