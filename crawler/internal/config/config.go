package config

import "github.com/zeromicro/go-zero/zrpc"

type Config struct {
	Ocean          zrpc.RpcClientConf
	DHTListen      string
	BootstrapNodes []string
	MaxQueueSize   int
}
