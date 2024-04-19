package config

import (
	"time"

	"github.com/zeromicro/go-zero/core/proc"
	"github.com/zeromicro/go-zero/core/service"
)

type Config struct {
	service.ServiceConf
	AMQP                string
	AMQPPreFetch        int `json:",default=64"`
	Mongo               string
	DHTListen           string `json:",default=:6881"`
	BootstrapNodes      []string
	FindNodeRateLimit   int    `json:",default=3000"`
	MaxQueueSize        int    `json:",default=3000"`
	SeenNodeTTL         int    `json:",default=300"`
	MaxSeenNodeSize     int    `json:",default=2048576"`
	NodeID              string `json:",optional"`
	TorrentWorkers      int    `json:",default=100"`
	TorrentMaxQueueSize int    `json:",default=1000"`
	BloomFilterPath     string `json:",default=bloom.json"`
	ForceQuitSeconds    int    `json:",default=20"`
	CheckExistBatchSize int    `json:",default=50"`
	Socks5Proxy         string `json:",optional"`
	Proxy               string `json:",optional"`
	ProxyBufSize        int    `json:",default=4096"`
	DecoderThreads      int    `json:",default=1"`
	HandlerThreads      int    `json:",default=1"`
	PacketQueueSize     int    `json:",default=2000"`
}

func (c *Config) MustSetUp() {
	c.ServiceConf.MustSetUp()
	proc.SetTimeToForceQuit(time.Duration(c.ForceQuitSeconds) * time.Second)
}
