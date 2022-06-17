package connpool

import (
	"errors"
	"sync"
	"sync/atomic"
)

var (
	ConnectError = errors.New("connect error")
)

type Closeable interface {
	Close()
}

// ConnectPoll 构建基础的连接池
// 支持 最大连接数 配置
type ConnectPoll struct {
	New      func() Closeable
	ch       chan Closeable
	using    int32
	maxCount int32
	l        sync.Mutex
}

func NewConnectPoll(maxCount int32, f func() Closeable) *ConnectPoll {
	ch := make(chan Closeable, maxCount)
	return &ConnectPoll{
		New:      f,
		maxCount: maxCount,
		ch:       ch,
	}
}

func (c *ConnectPoll) Call(f func(closeable Closeable) error) error {
	c.l.Lock()
	conn := c.get()
	atomic.AddInt32(&c.using, 1)
	c.l.Unlock()

	if nil == conn {
		atomic.AddInt32(&c.using, -1)
		return ConnectError
	}

	err := f(conn)

	if err != nil && errors.Is(err, ConnectError) {
		conn.Close()
	} else {
		success := c.put(conn)
		if !success {
			conn.Close()
		}
	}
	atomic.AddInt32(&c.using, -1)
	return err
}

// get 从连接池获取连接
func (c *ConnectPoll) get() Closeable {
	select {
	case ch := <-c.ch:
		return ch
	default:
		if c.using >= c.maxCount {
			ch := <-c.ch
			return ch
		} else {
			return c.New()
		}
	}
}

// put 把一个连接放入连接池
func (c *ConnectPoll) put(o Closeable) bool {
	select {
	case c.ch <- o:
		return true
	default:
		return false
	}
}
