package main

import (
	"context"
	"so-omnichain-example/connpool"
	"sync"
)

var (
	conns       map[string]*connpool.EvmConnectPoll
	connMapLock sync.Mutex
)

func init() {
	conns = make(map[string]*connpool.EvmConnectPoll)
}

func getConnectPool(rpcUrl string) *connpool.EvmConnectPoll {
	connMapLock.Lock()
	defer connMapLock.Unlock()
	if p, ok := conns[rpcUrl]; ok {
		return p
	}
	conns[rpcUrl] = connpool.NewEvmConnectPoll(context.Background(), rpcUrl, 2)
	return conns[rpcUrl]
}
