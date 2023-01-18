#!/bin/bash

MODULE=$1

goctl rpc protoc $1/$1.proto --go_out=$1 --go-grpc_out=$1 --zrpc_out=$1 -m