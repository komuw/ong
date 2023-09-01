package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/komuw/ong/config"
	"github.com/komuw/ong/internal/tst"
	"github.com/komuw/ong/log"
	"github.com/komuw/ong/middleware"

	"go.akshayshah.org/attest"
)

// The current [httptest.ResponseRecorder] does not work well with [http.ResponseController]
// Create one that does, until/unless issue is resolved upstream.
//
// See: https://github.com/golang/go/issues/60229
type responseControllerResponseRecorder struct {
	*httptest.ResponseRecorder
	// ReadDeadline is the last read deadline that has been set using [http.ResponseController]
	ReadDeadline time.Time

	// WriteDeadline is the last write deadline that has been set using [http.ResponseController]
	WriteDeadline time.Time
}

func (r *responseControllerResponseRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseRecorder // for Flush
}

func (r *responseControllerResponseRecorder) SetReadDeadline(deadline time.Time) error {
	r.ReadDeadline = deadline
	return nil
}

func (r *responseControllerResponseRecorder) SetWriteDeadline(deadline time.Time) error {
	r.WriteDeadline = deadline
	return nil
}

func TestPprofHandler(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		l := log.New(context.Background(), &bytes.Buffer{}, 500)
		port := tst.GetPort()

		o := config.WithOpts(
			"localhost",
			port,
			tst.SecretKey(),
			middleware.DirectIpStrategy,
			l,
		)

		h := pprofHandler(o)
		rec := &responseControllerResponseRecorder{ResponseRecorder: httptest.NewRecorder()}
		req := httptest.NewRequest(http.MethodGet, "//debug/pprof", nil)
		h.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		fmt.Println("body: ", string(rb)) // TODO:
		attest.Equal(t, res.StatusCode, http.StatusOK)
		// attest.Equal(t, string(rb), msg)
	})
}
