// Package finger provides(in a best effort manner) a client's TLS fingerprint.
package finger

import "sync/atomic"

// Print stores a TLS fingerprint.
// See [github.com/komuw/ong/middleware.ClientFingerPrint]
type Print struct {
	Hash atomic.Pointer[string]
}
