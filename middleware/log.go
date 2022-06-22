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

				fmt.Printf(`
				\n\t Error:
				requestAddr: %s,
				method: %s,
				path: %s,
				code: %d,
				status: %s,
				durationMS: %d,
				bytes: %d,
			`,
					r.RemoteAddr,
					r.Method,
					r.URL.EscapedPath(),
					lrw.code,
					http.StatusText(lrw.code),
					time.Now().Sub(start).Milliseconds(),
					lrw.sent,
				)
			} else {
				logger.Info(
					log.F{
						"requestAddr": r.RemoteAddr,
						"method":      r.Method,
						"path":        r.URL.EscapedPath(),
						"code":        lrw.code,
						"status":      http.StatusText(lrw.code),
						"durationMS":  time.Now().Sub(start).Milliseconds(),
						"bytes":       lrw.sent,
					},
				)
				logger.Error(errors.New("some-bad-error")) // TODO: remove this.
			}
		}()

		wrappedHandler(lrw, r)
	}
}

// logRW provides an http.ResponseWriter interface, which logs requests/responses.
type logRW struct {
	http.ResponseWriter

	// Code is the HTTP response code set by WriteHeader.
	// It is used to save this value for logging purposes.
	//
	// Note that if a Handler never calls WriteHeader or Write,
	// this might end up being 0, rather than the implicit
	// http.StatusOK. To get the implicit value, use the Result
	// method.
	code int
	// sent saves bytes sent
	sent int
}

// Write recodes the size of bytes sent for logging purposes.
func (lrw *logRW) Write(b []byte) (int, error) {
	if lrw.code == 0 {
		lrw.code = http.StatusOK
	}
	lrw.sent = len(b)
	return lrw.ResponseWriter.Write(b)
}

// WriteHeader recodes the status code for logging purposes.
func (lrw *logRW) WriteHeader(statusCode int) {
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
