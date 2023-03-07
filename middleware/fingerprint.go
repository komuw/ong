package middleware

import (
	"context"
	"net/http"
	"sync/atomic"

	"github.com/komuw/ong/internal/octx"
)

// TODO: docs.
type Fingerprint struct {
	Val atomic.Pointer[string]
}

// TODO: docs
func fingerprint(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		fHash := ""

		if vCtx := ctx.Value(octx.FingerPrintCtxKey); vCtx != nil {
			if s, ok := vCtx.(*Fingerprint); ok {
				if hash := s.Val.Load(); hash != nil {
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

// TODO: add docs.
func ClientFingerPrint(r *http.Request) string {
	ctx := r.Context()

	if vCtx := ctx.Value(octx.FingerPrintCtxKey); vCtx != nil {
		if s, ok := vCtx.(string); ok {
			return s
		}
	}

	return ""
}
