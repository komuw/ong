package server

import (
	"net"
	"sync/atomic"
)

// ///////////////////////////////////////////////////
type komuListener struct {
	inner net.Listener
}

func (l *komuListener) Accept() (net.Conn, error) {
	c, err := l.inner.Accept()
	if err != nil {
		return nil, err
	}
	return &komuConn{Conn: c}, nil
}

func (l *komuListener) Close() error {
	return l.inner.Close()
}

func (l *komuListener) Addr() net.Addr {
	return l.inner.Addr()
}

type Fingerprint struct {
	Val atomic.Pointer[string]
}

type komuConn struct {
	net.Conn
	fingerprint atomic.Pointer[Fingerprint]
}

var (
	_ net.Listener = &komuListener{}
	_ net.Conn     = &komuConn{}
)

type fingerPrintKeyType string

const (
	FingerPrintCtxKey = fingerPrintKeyType("fingerPrintKeyType")
)
