package config

import "github.com/zeromicro/go-zero/zrpc"

type Config struct {
	Ocean        zrpc.RpcClientConf
	Tracker      string
	TrackerLimit int64
}
