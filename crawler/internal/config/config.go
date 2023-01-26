package config

import (
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	service.ServiceConf
	Ocean               zrpc.RpcClientConf
	Redis               redis.RedisConf
	DHTListen           string `json:",default=:6881"`
	BootstrapNodes      []string
	FindNodeRateLimit   int    `json:",default=3000"`
	MaxQueueSize        int    `json:",default=3000"`
	NodeID              string `json:",optional"`
	TorrentWorkers      int    `json:",default=100"`
	TorrentMaxQueueSize int    `json:",default=1000"`
}

func (c *Config) MustSetUp() {
	c.ServiceConf.MustSetUp()
}
