package middleware

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/komuw/goweb/log"
)

// Log is a middleware that logs requests/responses.
func Log(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	// We want different requests to share the same logger backed by the same circular buffere for storing logs.
	// That way, if someone makes one request and it succeds then they make another one that errors.
	// When the logs are been flushed for the request that errored, the logs for the request that succeeded will also be flushed.
	// Thus app developers can be able to correlate issues/logs in much better way.
	//
	// However, each request should get its own context. That's why we call `logger.WithCtx` for every request.
	logger := log.New(
		context.Background(),
		os.Stderr,
		// TODO: increase maxMsgs
		5,
		// TODO: should we set indent to true/false?
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
			flds := log.F{
				"requestAddr": r.RemoteAddr,
				"method":      r.Method,
				"path":        r.URL.EscapedPath(),
				"code":        lrw.code,
				"status":      http.StatusText(lrw.code),
				"durationMS":  time.Now().Sub(start).Milliseconds(),
				"bytes":       lrw.sent,
			}

			if lrw.code >= http.StatusBadRequest {
				// both client and server errors.
				logger.Error(nil, flds)
			} else {
				logger.Info(flds)
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
	lrw.code = statusCode
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
