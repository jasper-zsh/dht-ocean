package config

import (
	"time"

	"github.com/zeromicro/go-zero/core/proc"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	service.ServiceConf
	Ocean               zrpc.RpcClientConf
	DHTListen           string `json:",default=:6881"`
	BootstrapNodes      []string
	FindNodeRateLimit   int    `json:",default=3000"`
	MaxQueueSize        int    `json:",default=3000"`
	NodeID              string `json:",optional"`
	TorrentWorkers      int    `json:",default=100"`
	TorrentMaxQueueSize int    `json:",default=1000"`
	BloomFilterPath     string `json:",default=bloom.json"`
	ForceQuitSeconds    int    `json:",default=20"`
	CheckExistBatchSize int    `json:",default=50"`
}

func (c *Config) MustSetUp() {
	c.ServiceConf.MustSetUp()
	proc.SetTimeToForceQuit(time.Duration(c.ForceQuitSeconds) * time.Second)
}
