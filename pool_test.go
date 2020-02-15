package go_pool

import (
	"fmt"
	"log"
	"net"
	"testing"
	"time"
)

var (
	network = "tcp"
	address = "127.0.0.1:7777"
	factory = func() (interface{}, error) { return net.Dial(network, address) }
)

func init() {
	go simpleTCPServer()
	time.Sleep(time.Millisecond * 300)
}

func simpleTCPServer() {
	l, err := net.Listen(network, address)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}

		go func() {
			buffer := make([]byte, 256)
			conn.Read(buffer)
		}()
	}
}

func TestNew(t *testing.T) {
	p, err := NewChannelPool(&Config{
		InitialCap: 1,
		MaxCap:     2,
		Factory:    factory,
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
		ConcurrentBase:     2,
	})

	if err != nil {
		t.Errorf("The pool returned an error: %s", err.Error())
	}
	if a := p.Len(); a != 1 {
		t.Errorf("The pool available was %d but should be 1", a)
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
	if a := p.Len(); a != 0 {
		t.Errorf("The pool available was %d but should be 0", a)
	}

	// Put the conn
	err = p.Put(conn)
	if err != nil {
		t.Errorf("Close returned an error: %s", err.Error())
	}
	if a := p.Len(); a != 1 {
		t.Errorf("The pool available was %d but should be 1", a)
	}

	if _, err := conn.Get(); err != ErrConnClosed {
		t.Errorf("Expected error \"%s\" but got \"%v\"",
			ErrConnClosed.Error(), err)
	}

	// Put the conn again
	err = p.Put(conn)
	if err != ErrConnClosed {
		t.Errorf("Expected error \"%s\" but got \"%s\"",
			ErrConnClosed.Error(), err.Error())
	}

	// Get 4 conns
	c1, err1 := p.Get()
	c2, err2 := p.Get()
	c3, err3 := p.Get()
	_, err4 := p.Get()
	if err1 != nil {
		t.Errorf("Err1 was not nil: %s", err1.Error())
	}
	if err2 != nil {
		t.Errorf("Err2 was not nil: %s", err2.Error())
	}
	if err3 != nil {
		t.Errorf("Err3 was not nil: %s", err3.Error())
	}
	if err4 != nil {
		t.Errorf("Err3 was not nil: %s", err4.Error())
	}

	if a := p.Len(); a != 0 {
		t.Errorf("The pool available was %d but should be 0", a)
	}

	_, err5 := p.Get()
	if err5 != ErrPoolTimeout {
		t.Errorf("Expected error \"%s\" but got \"%v\"",
			ErrPoolTimeout.Error(), err5)
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

	if a := p.Len(); a != 2 {
		t.Errorf("The pool available was %d but should be 2", a)
	}
}

func TestPoolTimeout(t *testing.T) {
	p, _ := NewChannelPool(&Config{
		InitialCap: 1,
		MaxCap:     1,
		Factory:    factory,
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

	_, err1 := p.Get()
	if err1 != nil {
		t.Errorf("Err1 was not nil: %s", err1.Error())
	}
	if a := p.Len(); a != 0 {
		t.Errorf("The pool available was %d but should be 0", a)
	}

	_, err2 := p.Get()
	if err2 != nil {
		t.Errorf("Err3 was not nil: %s", err2.Error())
	}

	_, err3 := p.Get()
	if err3 != ErrPoolTimeout {
		t.Errorf("Expected error \"%s\" but got \"%s\"",
			ErrPoolTimeout.Error(), err3.Error())
	}
}

func TestIdleTimeout(t *testing.T) {
	p, _ := NewChannelPool(&Config{
		InitialCap: 1,
		MaxCap:     1,
		Factory:    factory,
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
	p, _ := NewChannelPool(&Config{
		InitialCap: 1,
		MaxCap:     2,
		Factory:    factory,
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
		ConcurrentBase:     1,
	})

	if a := p.Len(); a != 1 {
		t.Errorf("The pool available was %d but should be 1", a)
	}

	c1, e1 := p.Get()
	if e1 != nil {
		t.Errorf("Get returned an error: %s", e1.Error())
	}
	c2, e2 := p.Get()
	if e2 != nil {
		t.Errorf("Get returned an error: %s", e2.Error())
	}
	_, e3 := p.Get()
	if e3 != ErrPoolTimeout {
		t.Errorf("Expected error \"%s\" but got \"%v\"",
			ErrPoolTimeout.Error(), e3)
	}

	p.Put(c1)
	p.Put(c2)

	if a := p.Len(); a != 2 {
		t.Errorf("The pool available was %d but should be 2", a)
	}

	p.Release()

	if a := p.Len(); a != 0 {
		t.Errorf("The pool available was %d but should be 0", a)
	}

	//
	c1, e1 = p.Get()
	if e1 != nil {
		t.Errorf("Get returned an error: %s", e1.Error())
	}
	p.Release()
	if a := p.Len(); a != 0 {
		t.Errorf("The pool available was %d but should be 0", a)
	}

	p.Put(c1)

	_, e11 := p.Get()
	if e11 != nil {
		t.Errorf("Get returned an error: %s", e11.Error())
	}
	_, e22 := p.Get()
	if e22 != nil {
		t.Errorf("Get returned an error: %s", e22.Error())
	}

	_, e33 := p.Get()
	if e33 != ErrPoolTimeout {
		t.Errorf("Expected error \"%s\" but got \"%v\"",
			ErrPoolTimeout.Error(), e33)
	}

}

