package go_pool

import (
	"net"
	"testing"
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

	if _, err := conn.Get(); err != nil {
		t.Errorf("conn get conn returned an error: %s", err.Error())
	}

	if a := p.Len(); a != 2 {
		t.Errorf("The pool available was %d but should be 2", a)
	}

	//// Put the conn
	err = p.Put(conn)
	if err != nil {
		t.Errorf("Close returned an error: %s", err.Error())
	}
	if a := p.Len(); a != 3 {
		t.Errorf("The pool available was %d but should be 3", a)
	}

	//// Put the conn again
	err = p.Put(conn)
	if err != ErrConnClosed {
		t.Errorf("Expected error \"%s\" but got \"%s\"",
			ErrConnClosed.Error(), err.Error())
	}
}
