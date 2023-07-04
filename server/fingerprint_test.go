package server

import (
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"io"
	"sync/atomic"
	"testing"

	"github.com/komuw/ong/internal/finger"
	"go.akshayshah.org/attest"
)

func TestSetFingerprint(t *testing.T) {
	t.Parallel()

	hasher := md5.New()

	tests := []struct {
		name  string
		hello func() *tls.ClientHelloInfo
		want  func() string
	}{
		{
			name:  "not fingerConn",
			hello: func() *tls.ClientHelloInfo { return &tls.ClientHelloInfo{} },
			want:  func() string { return "" },
		},
		{
			name: "zero fingerConn",
			hello: func() *tls.ClientHelloInfo {
				return &tls.ClientHelloInfo{
					Conn: &fingerConn{},
				}
			},
			want: func() string { return "" },
		},

		{
			name: "ClientHelloInfo with no state",
			hello: func() *tls.ClientHelloInfo {
				fingerprint := atomic.Pointer[finger.Print]{}

				fPrint := fingerprint.Load()
				if fPrint == nil {
					fPrint = &finger.Print{}
					fingerprint.CompareAndSwap(nil, fPrint)
				}

				return &tls.ClientHelloInfo{
					Conn: &fingerConn{
						fingerprint: fingerprint,
					},
				}
			},
			want: func() string {
				hasher.Reset()
				s := "0,,,,"
				_, _ = io.WriteString(hasher, s)
				return hex.EncodeToString(hasher.Sum(nil))
			},
		},

		{
			name: "ClientHelloInfo with state",
			hello: func() *tls.ClientHelloInfo {
				fingerprint := atomic.Pointer[finger.Print]{}

				fPrint := fingerprint.Load()
				if fPrint == nil {
					fPrint = &finger.Print{}
					fingerprint.CompareAndSwap(nil, fPrint)
				}

				return &tls.ClientHelloInfo{
					Conn: &fingerConn{
						fingerprint: fingerprint,
					},
					SupportedVersions: []uint16{1, 2, 3, 0x0B},
					CipherSuites:      []uint16{45, 9999, 8},
					SupportedCurves:   []tls.CurveID{tls.CurveP256, tls.CurveP384},
					SupportedPoints:   []uint8{9},
				}
			},
			want: func() string {
				hasher.Reset()
				s := "3,45-9999-8,,23-24,9"
				_, _ = io.WriteString(hasher, s)
				return hex.EncodeToString(hasher.Sum(nil))
			},
		},
	}
	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := setFingerprint(tt.hello())
			attest.Equal(t, s, tt.want())
		})
	}
}
