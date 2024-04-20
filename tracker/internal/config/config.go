package config

import (
	"github.com/zeromicro/go-zero/core/service"
)

type Config struct {
	service.ServiceConf
	AMQP             string
	AMQPPreFetch     int `json:",default=64"`
	Mongo            string
	Tracker          string
	TrackerLimit     int64
	TrackerQueueSize int `json:",default=200"`
	TrackerQueueTTL  int `json:",default=30"`
}
