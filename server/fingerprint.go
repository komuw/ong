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
//   (a) https://github.com/bpowers/go-fingerprint-example whose license(ISC License) can be found here:     https://github.com/bpowers/go-fingerprint-example/blob/d411f76d221249bd19085eb4baeff6f5c45b24c9/LICENSE
//   (b) https://github.com/sleeyax/ja3rp whose license(MIT) can be found here:                              https://github.com/sleeyax/ja3rp/blob/v0.0.1/LICENSE
//   (c) https://github.com/lwthiker/ts1 whose license(MIT) can be found here:                               https://github.com/lwthiker/ts1/blob/v0.1.6/LICENSE
//   (d) https://github.com/salesforce/ja3 whose license(BSD 3-Clause) can be found here:                    https://github.com/salesforce/ja3/blob/382cd37ea2759bcfc5627d2d7071fe2466833e90/LICENSE.txt
//   (e) https://github.com/LeeBrotherston/tls-fingerprinting whose license(GNU GPL v3.0) can be found here: https://github.com/LeeBrotherston/tls-fingerprinting/blob/1.0.1/LICENCE
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

// setFingerprint adds a TLS fingerprint to the connection[net.Conn]
func setFingerprint(info *tls.ClientHelloInfo) {
	conn, ok := info.Conn.(*fingerConn)
	if !ok {
		return
	}

	// The algorithm used here is based mainly on ja3 hash.
	// Basically;
	//   md5(SSLVersion, Cipher, SSLExtension, EllipticCurve, EllipticCurvePointFormat)
	//
	// https://github.com/sleeyax/ja3rp/blob/v0.0.1/crypto/tls/common.go#L462
	// https://github.com/salesforce/ja3/blob/382cd37ea2759bcfc5627d2d7071fe2466833e90/python/ja3.py

	// TODO: use const array.
	greaseTable := map[uint16]bool{
		// The GREASE protocol is way where tls clients(like google-chrome) can generate
		// random TLS values(especially for TLS extensions and/or cipherSuites).
		// The client then sends this, together with normal request, to servers.
		// This is intended to keep servers honest and not to be too bound to the current TLS
		// 'standard'; ie, they should be open to future extensions.
		//
		// https://www.rfc-editor.org/rfc/rfc8701.html

		// (a) GREASE values reserved for cipher suites and ALPN:
		0x0A: true,
		0x1A: true,
		0x2A: true,
		0x3A: true,
		0x4A: true,
		0x5A: true,
		0x6A: true,
		0x7A: true,
		0x8A: true,
		0x9A: true,
		0xAA: true,
		0xBA: true,
		0xCA: true,
		0xDA: true,
		0xEA: true,
		0xFA: true,

		// (b) GREASE values reserved for extensions, named groups, signature algorithms, and versions:
		0x0A0A: true,
		0x1A1A: true,
		0x2A2A: true,
		0x3A3A: true,
		0x4A4A: true,
		0x5A5A: true,
		0x6A6A: true,
		0x7A7A: true,
		0x8A8A: true,
		0x9A9A: true,
		0xAAAA: true,
		0xBABA: true,
		0xCACA: true,
		0xDADA: true,
		0xEAEA: true,
		0xFAFA: true,

		// (c) GREASE values reserved for for PskKeyExchangeModes:
		0x0B: true,
		// 0x2A: true, // duplicate in the map.
		0x49: true,
		0x68: true,
		0x87: true,
		0xA6: true,
		0xC5: true,
		0xE4: true,

		// see: https://www.rfc-editor.org/rfc/rfc8701.html#name-grease-values
	}
	// GREASE values may be sent in;
	// (i)   supported_versions
	// (ii)  cipher_suites
	// (iii) extensions
	// (iv)  supported_groups
	// (v)   signature_algorithms/signature_algorithms_cert
	// (vi)  psk_key_exchange_modes
	// (vii) application_layer_protocol_negotiation
	//
	// see: https://www.rfc-editor.org/rfc/rfc8701.html#name-client-behavior

	s := ""
	ver := uint16(0)
	for _, v := range info.SupportedVersions {
		// SupportedVersions lists the TLS versions supported by the client.
		// For TLS versions less than 1.3, this is extrapolated from the max version advertised by the client
		// see: https://github.com/golang/go/blob/go1.20.2/src/crypto/tls/common.go#L434-L438

		if _, ok := greaseTable[v]; ok {
			continue
		}

		if v > ver {
			ver = v
		}
	}
	s += fmt.Sprintf("%d,", ver)

	vals := []string{}
	for _, v := range info.CipherSuites {
		if _, ok := greaseTable[v]; ok {
			continue
		}

		vals = append(vals, fmt.Sprintf("%d", v))
	}
	s += fmt.Sprintf("%s,", strings.Join(vals, "-"))

	// Go currently does not expose `tls.ClientHelloInfo.extensions`.
	// This could be fixed if https://github.com/golang/go/issues/32936 is ever implemented.
	// Tracked in: https://github.com/komuw/ong/issues/194
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
