package agent

import (
	"fmt"
	"net"
)

// ServeTCP listens on the given address for a single incoming connection to
// start a payment channel.
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
	fmt.Fprintf(a.logWriter, "accepted connection from %v\n", conn.RemoteAddr())
	a.conn = conn
	err = a.hello()
	if err != nil {
		return fmt.Errorf("sending hello: %w", err)
	}
	go a.receiveLoop()
	return nil
}

// ConnectTCP connects to the given address for establishing a single payment
// channel.
func (a *Agent) ConnectTCP(addr string) error {
	if a.conn != nil {
		return fmt.Errorf("already connected")
	}
	var err error
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("connecting to %s: %w", addr, err)
	}
	fmt.Fprintf(a.logWriter, "connected to %v\n", conn.RemoteAddr())
	a.conn = conn
	err = a.hello()
	if err != nil {
		return fmt.Errorf("sending hello: %w", err)
	}
	go a.receiveLoop()
	return nil
}
