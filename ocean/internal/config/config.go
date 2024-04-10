package config

import (
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	Mongo            string
	ElasticSearch    string
	IndexerBatchSize int64
	BloomFilterPath  string `json:",default=bloom.json"`
}
