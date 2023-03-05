package server

import (
	"net"
	"sync/atomic"
)

type fingerPrintKeyType string

const FingerPrintCtxKey = fingerPrintKeyType("fingerPrintKeyType")

var (
	_ net.Listener = &fingerListener{}
	_ net.Conn     = &fingerConn{}
)

// fingerListener is a [net.Listener] that enables collection of a TLS fingerprint.
type fingerListener struct {
	net.Listener
}

func (l *fingerListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &fingerConn{Conn: c}, nil
}

type Fingerprint struct {
	Val atomic.Pointer[string]
}

// fingerConn is a [net.Conn] that enables collection of a TLS fingerprint.
type fingerConn struct {
	net.Conn
	fingerprint atomic.Pointer[Fingerprint]
}
