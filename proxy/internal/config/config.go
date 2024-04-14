package config

import "github.com/zeromicro/go-zero/core/service"

type Config struct {
	service.ServiceConf
	Listen     string
	BufferSize int `json:",default=4096"`
}
