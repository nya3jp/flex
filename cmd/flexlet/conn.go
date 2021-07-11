package main

import (
	"io"
	"net"
	"sync"
)

type watchedConn struct {
	net.Conn
	once sync.Once
	wg   *sync.WaitGroup
}

func (c *watchedConn) Close() error {
	c.once.Do(c.wg.Done)
	return c.Conn.Close()
}

type fixedListener struct {
	ch <-chan net.Conn
}

func newFixedListener(conns ...net.Conn) *fixedListener {
	ch := make(chan net.Conn, len(conns))
	var wg sync.WaitGroup
	wg.Add(len(conns))
	for _, conn := range conns {
		wconn := &watchedConn{Conn: conn, wg: &wg}
		ch <- wconn
	}
	go func() {
		wg.Wait()
		close(ch)
	}()
	return &fixedListener{ch}
}

func (l *fixedListener) Accept() (net.Conn, error) {
	conn, ok := <-l.ch
	if !ok {
		return nil, io.EOF
	}
	return conn, nil
}

func (l *fixedListener) Close() error {
	return nil
}

func (l *fixedListener) Addr() net.Addr {
	return &net.IPAddr{}
}

var _ net.Listener = &fixedListener{}
