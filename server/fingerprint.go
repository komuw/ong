package server

import (
	"net"
	"sync/atomic"
)

type fingerPrintKeyType string

const FingerPrintCtxKey = fingerPrintKeyType("fingerPrintKeyType")

var (
	_ net.Listener = &fingerListener{}
	_ net.Conn     = &komuConn{}
)

type fingerListener struct {
	net.Listener
}

func (l *fingerListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &komuConn{Conn: c}, nil
}

type Fingerprint struct {
	Val atomic.Pointer[string]
}

type komuConn struct {
	net.Conn
	fingerprint atomic.Pointer[Fingerprint]
}
