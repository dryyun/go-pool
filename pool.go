package go_pool

import (
	"errors"
	"time"
)

var (
	ErrPoolClosed = errors.New("pool is closed")

	ErrPoolTimeout = errors.New("poll get conn timed out")

	ErrConnClosed = errors.New("conn is closed")

	ErrConnGenerateFailed = errors.New("conn generate failed")

	ErrWrapConnNil = errors.New("wrap conn is nil. rejecting")
)

var (
	PoolTimeoutInit = time.Second
	IdleCheckInit = 30 * time.Minute
)

type WrapConn interface {
	Get() (interface{}, error)

	Close() error
}

// Pool 基本方法
type Pool interface {
	// 获取 WrapConn
	Get() (WrapConn, error)

	Put(WrapConn) error

	// 关闭单连接 idleConn
	Close(WrapConn) error

	// 释放连接池中所有连接
	Release()

	Ping(WrapConn) error

	Len() int
}
