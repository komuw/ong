package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/komuw/goweb/log"
)

// Log is a middleware that logs requests/responses.
func Log(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	// TODO: do we need a logger per request?
	logger := log.New(
		context.Background(),
		os.Stderr,
		// TODO: increase maxMsgs
		3,
		true,
	)
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger = logger.WithCtx(ctx)
		start := time.Now()

		// TODO: set cookie for logID.

		lrw := &logRW{
			ResponseWriter: w,
		}

		defer func() {
			if lrw.code >= http.StatusBadRequest {
				// both client and server errors.
				//
				// TODO: log at error.
				// logger.Error(e error)
			} else {
				logger.Info(
					log.F{
						"requestAddr": r.RemoteAddr,
						"method":      r.Method,
						"path":        r.URL.EscapedPath(),
						"code":        lrw.code,
						"status":      http.StatusText(lrw.code),
						"durationMS":  time.Now().Sub(start).Milliseconds(),
					},
				)
				logger.Error(errors.New("some-bad-error")) // TODO: remove this.
			}
		}()

		wrappedHandler(lrw, r)

		fmt.Println("\n\t at end: ", lrw)
	}
}

// logRW provides an http.ResponseWriter interface, which logs requests/responses.
type logRW struct {
	http.ResponseWriter
	code int // Saves the WriteHeader value.
}

// Write recodes the size of bytes sent for logging purposes.
func (lrw *logRW) Write(b []byte) (int, error) {
	fmt.Println("\n\t Write called: ", len(b))

	return lrw.ResponseWriter.Write(b)
}

// WriteHeader recodes the status code for logging purposes.
func (lrw *logRW) WriteHeader(statusCode int) {
	fmt.Println("\n\t WriteHeader called: ", statusCode)
	if lrw.code == 0 {
		lrw.code = statusCode
	}
	lrw.ResponseWriter.WriteHeader(statusCode)
}

// TODO: fix this.
var (
	// make sure we support http optional interfaces.
	// https://github.com/komuw/goweb/issues/15
	// https://blog.merovius.de/2017/07/30/the-trouble-with-optional-interfaces.html
	_ http.ResponseWriter = &logRW{}
	// _ http.Flusher        = &logRW{}
	// _ http.Hijacker       = &logRW{}
	// _ io.WriteCloser      = &logRW{}
	// _ http.CloseNotifier  = &logRW{} // `http.CloseNotifier` has been deprecated sinc Go v1.11(year 2018)
)
