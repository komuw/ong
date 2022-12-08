package middleware

import (
	"bufio"
	"context"
	"fmt"
	"io"
	mathRand "math/rand"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/komuw/ong/cookie"
	"github.com/komuw/ong/log"
)

const logIDKey = string(log.CtxKey)

// Log is a middleware that logs http requests and responses using [log.Logger].
func Log(wrappedHandler http.HandlerFunc, domain string, l log.Logger) http.HandlerFunc {
	// We pass the logger as an argument so that the middleware can share the same logger as the app.
	// That way, if the app logs an error, the middleware logs are also flushed.
	// This makes debugging easier for developers.
	//
	// However, each request should get its own context. That's why we call `logger.WithCtx` for every request.

	mathRand.Seed(time.Now().UTC().UnixNano())
	pid := os.Getpid()

	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		reqL := l.WithCtx(ctx)

		{
			// set cookie/headers/ctx for logID.
			logID := getLogId(r)
			ctx = context.WithValue(
				ctx,
				// using this custom key is important, instead of using `logIDKey`
				log.CtxKey,
				logID,
			)
			r = r.WithContext(ctx)
			r.Header.Set(logIDKey, logID)
			w.Header().Set(logIDKey, logID)
			cookie.Set(
				w,
				logIDKey,
				logID,
				domain,
				// Hopefully 15mins is enough.
				// Google considers a session to be 30mins.
				// https://support.google.com/analytics/answer/2731565?hl=en#time-based-expiration
				15*time.Minute,
				false,
			)
		}

		start := time.Now()
		lrw := &logRW{
			ResponseWriter: w,
		}
		defer func() {
			clientAddress := r.RemoteAddr
			if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
				clientAddress = host
			}

			flds := log.F{
				"clientAddress": clientAddress,
				"method":        r.Method,
				"path":          r.URL.Redacted(),
				"code":          lrw.code,
				"status":        http.StatusText(lrw.code),
				"durationMS":    time.Since(start).Milliseconds(),
				"pid":           pid,
			}
			if ongError := lrw.Header().Get(ongMiddlewareErrorHeader); ongError != "" {
				flds["ongError"] = ongError
			}

			// Remove header so that users dont see it.
			//
			// Note that this may not actually work.
			// According to: https://pkg.go.dev/net/http#ResponseWriter
			// Changing the header map after a call to WriteHeader (or
			// Write) has no effect unless the HTTP status code was of the
			// 1xx class or the modified headers are trailers.
			lrw.Header().Del(ongMiddlewareErrorHeader)

			if lrw.code == http.StatusServiceUnavailable || lrw.code == http.StatusTooManyRequests && w.Header().Get(retryAfterHeader) != "" {
				// We are either in load shedding or rate-limiting.
				// Only log 10% of the errors.
				shouldLog := mathRand.Intn(100) > 90
				if shouldLog {
					reqL.Error(nil, flds)
				}
			} else if lrw.code >= http.StatusBadRequest {
				// both client and server errors.
				reqL.Error(nil, flds)
			} else {
				reqL.Info(flds)
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

// Write recodes the size of bytes sent for logging purposes.
func (lrw *logRW) Write(b []byte) (int, error) {
	if lrw.code == 0 {
		lrw.code = http.StatusOK
	}
	return lrw.ResponseWriter.Write(b)
}

// WriteHeader recodes the status code for logging purposes.
func (lrw *logRW) WriteHeader(statusCode int) {
	lrw.code = statusCode
	lrw.ResponseWriter.WriteHeader(statusCode)
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

// getLogId returns a logID from the request or autogenerated if not available from the request.
func getLogId(req *http.Request) string {
	fromHeader := func(r *http.Request) string {
		if r != nil {
			if hdr := r.Header.Get(logIDKey); hdr != "" {
				return hdr
			}
		}
		return ""
	}

	fromCookie := func(r *http.Request) string {
		if r != nil {
			var cookies []*http.Cookie
			for _, v := range r.Cookies() {
				if v.Name == logIDKey && v.Value != "" {
					cookies = append(cookies, v)
				}
			}
			// there can be multiple cookies with the same name. get the latest
			if len(cookies) > 0 {
				return cookies[len(cookies)-1].Value
			}
		}
		return ""
	}

	fromCtx := func(ctx context.Context) string {
		return log.GetId(ctx)
	}

	// get logid in order of preference;
	// - http headers
	// - cookie
	// - context.Context

	if logID := fromHeader(req); logID != "" {
		return logID
	}

	if logID := fromCookie(req); logID != "" {
		return logID
	}

	return fromCtx(req.Context())
}
