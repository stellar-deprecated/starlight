package agent

import (
	"fmt"
	"net"
)

func (a *Agent) ServeTCP(addr string) error {
	if a.conn != nil {
		return fmt.Errorf("already connected")
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", addr, err)
	}
	conn, err := ln.Accept()
	if err != nil {
		return fmt.Errorf("accepting incoming connection: %w", err)
	}
	fmt.Fprintf(a.LogWriter, "accepted connection from %v\n", conn.RemoteAddr())
	a.conn = conn
	err = a.hello()
	if err != nil {
		return fmt.Errorf("sending hello: %w", err)
	}
	go a.loop()
	return nil
}

func (a *Agent) ConnectTCP(addr string) error {
	if a.conn != nil {
		return fmt.Errorf("already connected")
	}
	var err error
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("connecting to %s: %w", addr, err)
	}
	fmt.Fprintf(a.LogWriter, "connected to %v\n", conn.RemoteAddr())
	a.conn = conn
	err = a.hello()
	if err != nil {
		return fmt.Errorf("sending hello: %w", err)
	}
	go a.loop()
	return nil
}
