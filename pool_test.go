package go_pool

import (
	"fmt"
	"net"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	p, err := NewChannelPool(&Config{
		InitialCap: 1,
		MaxCap:     3,
		Factory: func() (i interface{}, err error) {
			return net.Dial("tcp", "example.com:http")
		},
		Close: func(i interface{}) error {
			if v, ok := i.(net.Conn); ok {
				return v.Close()
			}
			return nil
		},
		Ping:               nil,
		IdleTimeout:        0,
		PoolTimeout:        0,
		IdleCheckFrequency: 0,
	})

	if err != nil {
		t.Errorf("The pool returned an error: %s", err.Error())
	}
	if a := p.Len(); a != 3 {
		t.Errorf("The pool available was %d but should be 3", a)
	}

	// Get a conn
	conn, err := p.Get()
	if err != nil {
		t.Errorf("Get returned an error: %s", err.Error())
	}
	if conn == nil {
		t.Error("conn was nil")
	}
	if c, err := conn.Get(); err != nil || c == nil {
		t.Errorf("conn get conn returned an error: %s", err.Error())
	}
	if a := p.Len(); a != 2 {
		t.Errorf("The pool available was %d but should be 2", a)
	}

	// Put the conn
	err = p.Put(conn)
	if err != nil {
		t.Errorf("Close returned an error: %s", err.Error())
	}
	if a := p.Len(); a != 3 {
		t.Errorf("The pool available was %d but should be 3", a)
	}

	if _, err := conn.Get(); err != ErrConnClosed {
		t.Errorf("Expected error \"%s\" but got \"%s\"",
			ErrConnClosed.Error(), err.Error())
	}

	// Put the conn again
	err = p.Put(conn)
	if err != ErrConnClosed {
		t.Errorf("Expected error \"%s\" but got \"%s\"",
			ErrConnClosed.Error(), err.Error())
	}

	// Get 3 conns
	c1, err1 := p.Get()
	c2, err2 := p.Get()
	c3, err3 := p.Get()
	if err1 != nil {
		t.Errorf("Err1 was not nil: %s", err1.Error())
	}
	if err2 != nil {
		t.Errorf("Err2 was not nil: %s", err2.Error())
	}
	if err3 != nil {
		t.Errorf("Err3 was not nil: %s", err3.Error())
	}

	if a := p.Len(); a != 0 {
		t.Errorf("The pool available was %d but should be 0", a)
	}

	_, err4 := p.Get()
	if err4 != ErrPoolTimeout {
		t.Errorf("Expected error \"%s\" but got \"%s\"",
			ErrPoolTimeout.Error(), err4.Error())
	}

	// Put all of conns
	err1 = p.Put(c1)
	if err1 != nil {
		t.Errorf("Close returned an error: %s", err1.Error())
	}
	err2 = p.Put(c2)
	if err2 != nil {
		t.Errorf("Close returned an error: %s", err2.Error())
	}
	err3 = p.Put(c3)
	if err3 != nil {
		t.Errorf("Close returned an error: %s", err3.Error())
	}
}

func TestPoolTimeout(t *testing.T) {
	p, err := NewChannelPool(&Config{
		InitialCap: 1,
		MaxCap:     1,
		Factory: func() (i interface{}, err error) {
			return net.Dial("tcp", "example.com:http")
		},
		Close: func(i interface{}) error {
			if v, ok := i.(net.Conn); ok {
				return v.Close()
			}
			return nil
		},
		Ping:               nil,
		IdleTimeout:        0,
		PoolTimeout:        3 * time.Second,
		IdleCheckFrequency: 0,
	})

	if err != nil {
		t.Errorf("The pool returned an error: %s", err.Error())
	}

	_, err1 := p.Get()
	if err1 != nil {
		t.Errorf("Err1 was not nil: %s", err1.Error())
	}
	if a := p.Len(); a != 0 {
		t.Errorf("The pool available was %d but should be 0", a)
	}

	_, err2 := p.Get()
	if err2 != ErrPoolTimeout {
		t.Errorf("Expected error \"%s\" but got \"%s\"",
			ErrPoolTimeout.Error(), err2.Error())
	}
}

func TestIdleTimeout(t *testing.T) {
	p, err := NewChannelPool(&Config{
		InitialCap: 1,
		MaxCap:     1,
		Factory: func() (i interface{}, err error) {
			return net.Dial("tcp", "example.com:http")
		},
		Close: func(i interface{}) error {
			if v, ok := i.(net.Conn); ok {
				return v.Close()
			}
			return nil
		},
		Ping:               nil,
		IdleTimeout:        5 * time.Second,
		PoolTimeout:        0,
		IdleCheckFrequency: 0,
	})

	if err != nil {
		t.Errorf("The pool returned an error: %s", err.Error())
	}

	// 以下在一次获取 conn 的过程中，wrapconn 地址会变，conn 的地址不变

	wrapConn, wrapConnErr := p.Get() //

	if wrapConnErr != nil {
		t.Errorf("Get returned an error: %s", wrapConnErr.Error())
	}

	wrapConnP1 := fmt.Sprintf("%p", wrapConn) // 获取 wrapConn 的指针地址

	conn, connErr := wrapConn.Get()
	if connErr != nil {
		t.Errorf("Get returned an error: %s", connErr.Error())
	}
	connP1 := fmt.Sprintf("%p", conn) // 获取 conn 的指针地址

	p.Put(wrapConn)

	wrapConn, wrapConnErr = p.Get() //
	if wrapConnErr != nil {
		t.Errorf("Get returned an error: %s", wrapConnErr.Error())
	}
	wrapConnP2 := fmt.Sprintf("%p", wrapConn) // 再次获取 wrapConn 的指针地址

	if wrapConnP1 == wrapConnP2 { // 应该不相等
		t.Error("wrapConn ptr address is equal")
	}

	conn, connErr = wrapConn.Get()
	if connErr != nil {
		t.Errorf("Get returned an error: %s", connErr.Error())
	}
	connP2 := fmt.Sprintf("%p", conn) // 再次获取 conn 的指针地址

	if connP1 != connP2 { // 应该相等
		t.Error("conn ptr address is not equal")
	}

	p.Put(wrapConn)

	// 经过了一个 IdleTimeout 周期之后
	// conn 的地址也变了
	time.Sleep(5 * time.Second)

	wrapConn, wrapConnErr = p.Get() //
	if wrapConnErr != nil {
		t.Errorf("Get returned an error: %s", wrapConnErr.Error())
	}
	conn, connErr = wrapConn.Get()
	if connErr != nil {
		t.Errorf("Get returned an error: %s", connErr.Error())
	}
	connP3 := fmt.Sprintf("%p", conn) // 再次获取 conn 的指针地址

	if connP1 == connP3 {
		t.Error("conn ptr address is  equal")
	}

}

func TestChannelPool_Release(t *testing.T) {
	p, err := NewChannelPool(&Config{
		InitialCap: 3,
		MaxCap:     3,
		Factory: func() (i interface{}, err error) {
			return net.Dial("tcp", "example.com:http")
		},
		Close: func(i interface{}) error {
			if v, ok := i.(net.Conn); ok {
				return v.Close()
			}
			return nil
		},
		Ping:               nil,
		IdleTimeout:        0,
		PoolTimeout:        0,
		IdleCheckFrequency: 0,
	})

	if err != nil {
		t.Errorf("The pool returned an error: %s", err.Error())
	}
	if a := p.Len(); a != 3 {
		t.Errorf("The pool available was %d but should be 3", a)
	}

	p.Release()

	if a := p.Len(); a != 3 {
		t.Errorf("The pool available was %d but should be 3", a)
	}

}
