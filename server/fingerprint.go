package server

import (
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"sync/atomic"

	"github.com/komuw/ong/middleware"
)

// Most of the code here is inspired(or taken from) by:
//   (a) https://github.com/bpowers/go-fingerprint-example whose license(ISC License) can be found here: https://github.com/bpowers/go-fingerprint-example/blob/d411f76d221249bd19085eb4baeff6f5c45b24c9/LICENSE
//   (b) https://github.com/sleeyax/ja3rp whose license(MIT) can be found here:                          https://github.com/sleeyax/ja3rp/blob/v0.0.1/LICENSE
//   (c) https://github.com/lwthiker/ts1 whose license(MIT) can be found here:                           https://github.com/lwthiker/ts1/blob/v0.1.6/LICENSE
//

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

// fingerConn is a [net.Conn] that enables collection of a TLS fingerprint.
type fingerConn struct {
	net.Conn
	fingerprint atomic.Pointer[middleware.Fingerprint]
}

// TODO: docs.
// setFingerprint adds a TLS fingerprint to the connection.
func setFingerprint(info *tls.ClientHelloInfo) {
	conn, ok := info.Conn.(*fingerConn)
	if !ok {
		return
	}

	// SSLVersion,Cipher,SSLExtension,EllipticCurve,EllipticCurvePointFormat

	// TODO: check if this table is upto date and accurate.
	greaseTable := map[uint16]bool{
		0x0a0a: true, 0x1a1a: true, 0x2a2a: true, 0x3a3a: true,
		0x4a4a: true, 0x5a5a: true, 0x6a6a: true, 0x7a7a: true,
		0x8a8a: true, 0x9a9a: true, 0xaaaa: true, 0xbaba: true,
		0xcaca: true, 0xdada: true, 0xeaea: true, 0xfafa: true,
	}

	s := ""
	ver := uint16(0)
	for _, v := range info.SupportedVersions {
		// SupportedVersions lists the TLS versions supported by the client.
		// For TLS versions less than 1.3, this is extrapolated from the max version advertised by the client
		// see: https://github.com/golang/go/blob/go1.20.2/src/crypto/tls/common.go#L434-L438
		if v > ver {
			ver = v
		}
	}
	// TODO: use strings builder.
	s += fmt.Sprintf("%d,", ver)

	vals := []string{}
	for _, v := range info.CipherSuites {
		vals = append(vals, fmt.Sprintf("%d", v))
	}
	s += fmt.Sprintf("%s,", strings.Join(vals, "-"))

	// TODO: Explain this. Because `tls.ClientHelloInfo` does not have extensions.
	// This should be fixed if https://github.com/golang/go/issues/32936 is ever implemented.
	extensions := []uint16{}
	vals = []string{}
	for _, v := range extensions {
		if _, ok := greaseTable[v]; ok {
			continue
		}

		vals = append(vals, fmt.Sprintf("%d", v))
	}
	s += fmt.Sprintf("%s,", strings.Join(vals, "-"))

	vals = []string{}
	for _, v := range info.SupportedCurves {
		vals = append(vals, fmt.Sprintf("%d", v))
	}
	s += fmt.Sprintf("%s,", strings.Join(vals, "-"))

	vals = []string{}
	for _, v := range info.SupportedPoints {
		vals = append(vals, fmt.Sprintf("%d", v))
	}
	s += fmt.Sprintf("%s", strings.Join(vals, "-"))

	hasher := md5.New()
	hasher.Write([]byte(s))
	hash := hex.EncodeToString(hasher.Sum(nil))

	conn.fingerprint.Load().Hash.Store(&hash)
}
