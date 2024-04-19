package config

import (
	"github.com/zeromicro/go-zero/core/service"
)

type Config struct {
	service.ServiceConf
	AMQP          string
	AMQPPreFetch  int `json:",default=64"`
	Mongo         string
	ElasticSearch string
	BatchSize     int64
}
