package middleware

import (
	"bufio"
	"fmt"
	"io"
	mathRand "math/rand"
	"net"
	"net/http"
	"time"

	"github.com/komuw/ong/log"

	"golang.org/x/exp/slog"
)

// logger is a middleware that logs http requests and responses using [log.Logger].
func logger(wrappedHandler http.Handler, l *slog.Logger) http.HandlerFunc {
	// We pass the logger as an argument so that the middleware can share the same logger as the app.
	// That way, if the app logs an error, the middleware logs are also flushed.
	// This makes debugging easier for developers.
	//
	// However, each request should get its own context. That's why we call `logger.WithCtx` for every request.

	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &logRW{
			ResponseWriter: w,
		}
		defer func() {
			msg := "http_server"
			flds := []any{
				"clientIP", ClientIP(r),
				"clientFingerPrint", ClientFingerPrint(r),
				"method", r.Method,
				"path", r.URL.Redacted(),
				"code", lrw.code,
				"status", http.StatusText(lrw.code),
				"durationMS", time.Since(start).Milliseconds(),
			}
			if ongError := lrw.Header().Get(ongMiddlewareErrorHeader); ongError != "" {
				extra := []any{"ongError", ongError}
				flds = append(flds, extra...)
			}

			// Remove header so that users dont see it.
			//
			// Note that this may not actually work.
			// According to: https://pkg.go.dev/net/http#ResponseWriter
			// Changing the header map after a call to WriteHeader (or
			// Write) has no effect unless the HTTP status code was of the
			// 1xx class or the modified headers are trailers.
			lrw.Header().Del(ongMiddlewareErrorHeader)

			// The logger should be in the defer block so that it uses the updated context containing the logID.
			reqL := log.WithID(r.Context(), l)

			if lrw.code == http.StatusServiceUnavailable || lrw.code == http.StatusTooManyRequests && w.Header().Get(retryAfterHeader) != "" {
				// We are either in load shedding or rate-limiting.
				// Only log 10% of the errors.
				shouldLog := mathRand.Intn(100) > 90
				if shouldLog {
					reqL.Error(msg, flds...)
				}
			} else if lrw.code >= http.StatusBadRequest {
				// Both client and server errors.

				if lrw.code == http.StatusNotFound ||
					lrw.code == http.StatusMethodNotAllowed ||
					lrw.code == http.StatusConflict ||
					lrw.code == http.StatusTeapot {
					// These ones are more annoying, than been actual errors.
					reqL.Info(msg, flds...)
				} else {
					reqL.Error(msg, flds...)
				}
			} else {
				reqL.Info(msg, flds...)
			}
		}()

		wrappedHandler.ServeHTTP(lrw, r)
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
	code          int
	writtenHeader bool
}

var (
	// make sure we support http optional interfaces.
	// https://github.com/komuw/ong/issues/15
	// https://blog.merovius.de/2017/07/30/the-trouble-with-optional-interfaces.html
	_ http.ResponseWriter = &logRW{}
	_ http.Flusher        = &logRW{}
	_ http.Hijacker       = &logRW{}
	_ http.Pusher         = &logRW{}
	_ io.ReaderFrom       = &logRW{}
	// _ http.CloseNotifier  = &logRW{} // `http.CloseNotifier` has been deprecated sinc Go v1.11(year 2018)
)

// Write sets the default status code if not already set.
// todo: In future, it could also recode the size of bytes sent for logging purposes.
func (lrw *logRW) Write(b []byte) (int, error) {
	if lrw.code == 0 {
		// If WriteHeader is not called explicitly, the first call to Write
		// will trigger an implicit WriteHeader(http.StatusOK).
		// See: https://github.com/golang/go/blob/go1.20.5/src/net/http/server.go#L141-L159
		//
		// Thus here we need to obey that convention
		lrw.code = http.StatusOK
	}
	lrw.writtenHeader = true
	return lrw.ResponseWriter.Write(b)
}

// WriteHeader recodes the status code for logging purposes.
func (lrw *logRW) WriteHeader(statusCode int) {
	lrw.ResponseWriter.WriteHeader(statusCode)

	if !lrw.writtenHeader {
		lrw.code = statusCode
		lrw.writtenHeader = true
	}
}

// Flush implements http.Flusher
func (lrw *logRW) Flush() {
	if fw, ok := lrw.ResponseWriter.(http.Flusher); ok {
		fw.Flush()
	}
}

// Hijack implements http.Hijacker
func (lrw *logRW) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := lrw.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("ong/middleware: http.Hijacker interface is not supported")
}

// Push implements http.Pusher
func (lrw *logRW) Push(target string, opts *http.PushOptions) error {
	if p, ok := lrw.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return fmt.Errorf("ong/middleware: http.Pusher interface is not supported")
}

// ReadFrom implements io.ReaderFrom
// It is necessary for the sendfile syscall
// https://github.com/caddyserver/caddy/pull/5022
func (lrw *logRW) ReadFrom(src io.Reader) (n int64, err error) {
	if rf, ok := lrw.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(src)
	}
	return io.Copy(lrw.ResponseWriter, src)
}
