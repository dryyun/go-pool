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
	IdleCheckInit   = 30 * time.Minute
)

// Pool 基本方法
type Pool interface {
	// 获取 WrapConn
	Get() (*IdleConn, error)

	Put(*IdleConn) error

	// 关闭单连接 idleConn
	Close(*IdleConn) error

	// 释放连接池中所有连接
	Release()

	Ping(*IdleConn) error

	Len() int
}
