package connpool

import (
	"context"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

type EvmConnectPoll struct {
	*ConnectPoll
}

// NewEvmConnectPoll 初始化 evm websocket 连接池
func NewEvmConnectPoll(ctx context.Context, rawUrl string, maxConnect int) *EvmConnectPoll {
	return &EvmConnectPoll{
		ConnectPoll: NewConnectPoll(int32(maxConnect), func() Closeable {
			client, err := rpc.DialContext(ctx, rawUrl)
			if err != nil {
				return nil
			}
			return client
		}),
	}
}

func (e *EvmConnectPoll) Call(f func(*ethclient.Client, *rpc.Client) error) error {
	return e.ConnectPoll.Call(func(closeable Closeable) error {
		if client, ok := closeable.(*rpc.Client); ok {
			return f(ethclient.NewClient(client), client)
		} else {
			return ConnectError
		}
	})
}
