package go_pool

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Config 连接池相关配置
type Config struct {
	//连接池中拥有的最小连接数
	InitialCap int
	//连接池中拥有的最大的连接数
	MaxCap int
	//生成连接的方法
	Factory func() (interface{}, error)
	//关闭连接的方法
	Close func(interface{}) error
	//检查连接是否有效的方法
	Ping func(interface{}) error
	//连接最大空闲时间，超过该时间则将失效，根据上次使用时间判断，不设置不检查
	IdleTimeout time.Duration
	//获取连接的超时时间，不设置不检查
	PoolTimeout time.Duration // TODO .
	// conn 检测时间，默认 30m , -1 = disable
	IdleCheckFrequency time.Duration // TODO .
}

// channelPool 存放连接信息
type channelPool struct {
	mu                 sync.RWMutex
	poolSize           int
	queue              chan struct{} // 考虑存活的 conn 数量，可以是 poolSize 的几倍，需要控制 conn 的数量
	conns              chan *idleConn
	factory            func() (interface{}, error)
	close              func(interface{}) error
	ping               func(interface{}) error
	idleTimeout        time.Duration
	poolTimeout        time.Duration
	idleCheckFrequency time.Duration
}

type idleConn struct {
	mu   sync.RWMutex
	conn interface{}
	t    time.Time
}

func (i *idleConn) Get() (interface{}, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.conn == nil {
		return nil, ErrConnClosed
	}
	return i.conn, nil
}

func (i *idleConn) Close() error {
	i.mu.Lock()
	i.conn = nil
	i.t = time.Time{}
	i.mu.Unlock()
	return nil
}

// NewChannelPool 初始化连接
func NewChannelPool(poolConfig *Config) (Pool, error) {
	if poolConfig.InitialCap < 0 || poolConfig.MaxCap <= 0 || poolConfig.InitialCap > poolConfig.MaxCap {
		return nil, errors.New("invalid capacity settings")
	}
	if poolConfig.Factory == nil {
		return nil, errors.New("invalid factory func settings")
	}
	if poolConfig.Close == nil {
		return nil, errors.New("invalid close func settings")
	}

	if poolConfig.IdleCheckFrequency == 0 {
		poolConfig.IdleCheckFrequency = IdleCheckInit
	}

	c := &channelPool{
		conns:              make(chan *idleConn, poolConfig.MaxCap),
		queue:              make(chan struct{}, 3*poolConfig.MaxCap),
		factory:            poolConfig.Factory,
		close:              poolConfig.Close,
		idleTimeout:        poolConfig.IdleTimeout,
		poolTimeout:        poolConfig.PoolTimeout,
		idleCheckFrequency: poolConfig.IdleCheckFrequency,
	}

	if poolConfig.Ping != nil {
		c.ping = poolConfig.Ping
	}

	for i := 0; i < poolConfig.InitialCap; i++ {
		conn, err := c.generateConn()
		if err != nil {
			c.Release()
			return nil, fmt.Errorf("factory is not able to fill the pool: %s", err)
		}
		c.conns <- conn
	}

	//// 空闲连接处理
	//if c.idleCheckFrequency > 0 && c.idleTimeout > 0 {
	//	go c.reaper(c.idleCheckFrequency)
	//}

	return c, nil
}

//func (c *channelPool) reaper(frequency time.Duration) {
//	ticker := time.NewTicker(frequency)
//	defer ticker.Stop()
//
//	for range ticker.C {
//		conns := c.getConns()
//		if conns == nil {
//			break
//		}
//		c.reapStaleConns(conns)
//	}
//}
//
//func (c *channelPool) reapStaleConns(conns chan *idleConn) {
//	kcs := make([]*idleConn, 0, len(conns))
//
//	ll := len(conns)
//	index := 0
//
//	for wrapConn := range conns {
//		if idleTimeout := c.idleTimeout; idleTimeout > 0 && wrapConn.t.Add(idleTimeout).Before(time.Now()) {
//			c.Close(wrapConn)
//			wrapConn.conn = nil
//			kcs = append(kcs, &idleConn{})
//		} else {
//			kcs = append(kcs, wrapConn)
//		}
//		index += 1
//		if index == ll {
//			break
//		}
//	}
//
//	for _, kc := range kcs {
//		conns <- kc
//	}
//
//	return
//}

// getConns 获取所有连接
func (c *channelPool) getConns() chan *idleConn {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.conns
}

func (c *channelPool) generateConn() (*idleConn, error) {
	select {
	case c.queue <- struct{}{}:
		// get
	case <-time.After(c.poolTimeout):
		return nil, ErrPoolTimeout
	}

	conn, err := c.factory()
	if err != nil {
		c.freeTurn()
		return nil, ErrConnGenerateFailed
	}
	return &idleConn{
		conn: conn,
		t:    time.Now(),
	}, nil
}

func (c *channelPool) freeTurn() {
	<-c.queue
}

// Get 从 pool 中取一个连接
func (c *channelPool) Get() (WrapConn, error) {
	conns := c.getConns()
	if conns == nil {
		return nil, ErrPoolClosed
	}

	select {
	case wrapConn := <-conns:
		if wrapConn.conn != nil {
			//判断是否超时，超时则丢弃
			if idleTimeout := c.idleTimeout; idleTimeout > 0 && wrapConn.t.Add(idleTimeout).Before(time.Now()) {
				//丢弃并关闭该连接
				c.Close(wrapConn)
				wrapConn.conn = nil
			}

			//判断是否失效，失效则丢弃，如果用户没有设定 ping 方法，就不检查
			if err := c.Ping(wrapConn); err != nil {
				c.Close(wrapConn)
				wrapConn.conn = nil
			}
		}

		if wrapConn.conn == nil {
			return c.generateConn()
		}

		return wrapConn, nil
	default:
		return c.generateConn()
	}
}

// Put 将连接放回 pool 中
func (c *channelPool) Put(wrapConn WrapConn) error {
	if wrapConn == nil {
		return nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conns == nil {
		return c.Close(wrapConn)
	}

	conn, err := wrapConn.Get()
	if err != nil {
		c.Close(wrapConn)
		return err
	}

	select {
	case c.conns <- &idleConn{conn: conn, t: time.Now()}:
		return nil
	default:
		//连接池已满，直接关闭该连接
		c.Close(wrapConn)
		return nil
	}
}

// Close 关闭单条连接
func (c *channelPool) Close(wrapConn WrapConn) error {
	if wrapConn == nil {
		return nil
	}

	c.freeTurn()

	conn, err := wrapConn.Get()
	if err != nil {
		return err
	}
	wrapConn.Close()

	return c.close(conn)
}

// Ping 检查单条连接是否有效
func (c *channelPool) Ping(wrapConn WrapConn) error {
	if c.ping == nil {
		return nil
	}

	if wrapConn == nil {
		return ErrWrapConnNil
	}

	conn, err := wrapConn.Get()
	if err != nil {
		return err
	}

	return c.ping(conn)
}

// Release 释放连接池中所有连接
func (c *channelPool) Release() {
	c.mu.Lock()
	conns := c.conns
	c.conns = nil
	c.mu.Unlock()

	if conns == nil {
		return
	}

	close(conns)
	for conn := range conns {
		c.Close(conn)
	}
}

// Len 连接池中已有的连接
func (c *channelPool) Len() int {
	if c == nil {
		return 0
	}
	conns := c.getConns()
	if conns == nil {
		return 0
	}
	return len(conns)
}
