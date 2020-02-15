package go_pool

import (
	"sync"
	"time"
)

type IdleConn struct {
	mu   sync.RWMutex
	conn interface{}
	t    time.Time
	pool *channelPool
}

func (i *IdleConn) Get() (interface{}, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.conn == nil {
		return nil, ErrConnClosed
	}
	return i.conn, nil
}

func (i *IdleConn) Close() error {
	i.mu.Lock()
	i.conn = nil
	i.t = time.Time{}
	i.pool = nil
	i.mu.Unlock()
	return nil
}

func (i *IdleConn) GetPool() (Pool, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.pool == nil {
		return nil, ErrConnClosed
	}
	return i.pool, nil
}
