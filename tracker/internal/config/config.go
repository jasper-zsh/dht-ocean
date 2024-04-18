package config

import (
	"github.com/zeromicro/go-zero/core/service"
)

type Config struct {
	service.ServiceConf
	Mongo        string
	Tracker      string
	TrackerLimit int64
}
