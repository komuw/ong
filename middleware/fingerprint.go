package middleware

import (
	"context"
	"net/http"

	"github.com/komuw/ong/internal/finger"
	"github.com/komuw/ong/internal/octx"
)

// fingerprint is a middleware that adds the client's TLS fingerprint to the request context.
// The fingerprint can be fetched using [ClientFingerPrint]
func fingerprint(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		fHash := ""

		if vCtx := ctx.Value(octx.FingerPrintCtxKey); vCtx != nil {
			if s, ok := vCtx.(*finger.Print); ok {
				if hash := s.Hash.Load(); hash != nil {
					fHash = *hash
				}
			}
		}

		ctx = context.WithValue(
			ctx,
			octx.FingerPrintCtxKey,
			fHash,
		)
		r = r.WithContext(ctx)

		wrappedHandler(w, r)
	}
}

// ClientFingerPrint returns the [TLS fingerprint] of the client.
// It is provided on a best-effort basis.
// There are different formats of fingerprinting, this library does not subscribe to a particular format.
//
// [TLS fingerprint]: https://github.com/LeeBrotherston/tls-fingerprinting
func ClientFingerPrint(r *http.Request) string {
	ctx := r.Context()

	if vCtx := ctx.Value(octx.FingerPrintCtxKey); vCtx != nil {
		if s, ok := vCtx.(string); ok {
			return s
		}
	}

	return ""
}