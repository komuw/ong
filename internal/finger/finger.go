// Package finger provides(in a best effort manner) a client's TLS fingerprint.
package finger

import (
	"net/http"
	"sync/atomic"

	"github.com/komuw/ong/internal/octx"
)

// Print stores a TLS fingerprint.
// See [github.com/komuw/ong/middleware.ClientFingerPrint]
type Print struct {
	Hash atomic.Pointer[string]
}

// Get returns the [TLS fingerprint] of the client.
// See [github.com/komuw/ong/middleware.ClientFingerPrint]
func Get(r *http.Request) string {
	ctx := r.Context()

	if vCtx := ctx.Value(octx.FingerPrintCtxKey); vCtx != nil {
		if s, ok := vCtx.(string); ok {
			return s
		}
	}

	return ""
}
