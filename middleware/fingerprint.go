package middleware

import (
	"context"
	"net/http"

	"github.com/komuw/ong/internal/finger"
	"github.com/komuw/ong/internal/octx"
)

// fingerprint is a middleware that adds the client's TLS fingerprint to the request context.
// The fingerprint can be fetched using [ClientFingerPrint]
func fingerprint(wrappedHandler http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
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

			wrappedHandler.ServeHTTP(w, r)
		},
	)
}

// ClientFingerPrint returns the [TLS fingerprint] of the client.
// It is provided on a best-effort basis. If a fingerprint is not found, it returns a string that has the substring "NotFound" in it.
// There are different formats/algorithms of fingerprinting, this library(by design) does not subscribe to a particular format or algorithm.
//
// [TLS fingerprint]: https://github.com/LeeBrotherston/tls-fingerprinting
func ClientFingerPrint(r *http.Request) string {
	return finger.Get(r)
}
