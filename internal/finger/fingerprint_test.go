package finger

import (
	"context"
	"net/http"
	"testing"

	"github.com/komuw/ong/internal/octx"
	"go.akshayshah.org/attest"
)

func TestGet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  func() *http.Request
		want string
	}{
		{
			name: "no fingerprint",
			req: func() *http.Request {
				req, err := http.NewRequestWithContext(
					context.Background(),
					http.MethodGet,
					"/hey",
					nil,
				)
				attest.Ok(t, err)
				return req
			},
			want: notFound,
		},

		{
			name: "with fingerprint",
			req: func() *http.Request {
				ctx := context.WithValue(context.Background(), octx.FingerPrintCtxKey, "we4376t30lmbcflmh7a28d9d8eb38jk9")
				req, err := http.NewRequestWithContext(
					ctx,
					http.MethodGet,
					"/hey",
					nil,
				)
				attest.Ok(t, err)
				return req
			},
			want: "we4376t30lmbcflmh7a28d9d8eb38jk9",
		},

		{
			name: "zero value Print",
			req: func() *http.Request {
				// See: https://github.com/komuw/ong/blob/v0.0.59/server/server.go#L294-L298
				fPrint := &Print{}
				ctx := context.WithValue(context.Background(), octx.FingerPrintCtxKey, fPrint)
				req, err := http.NewRequestWithContext(
					ctx,
					http.MethodGet,
					"/hey",
					nil,
				)
				attest.Ok(t, err)
				return req
			},
			want: notFound,
		},
	}
	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := Get(tt.req())
			attest.Equal(t, f, tt.want)
		})
	}
}
