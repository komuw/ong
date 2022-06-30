package middleware

import (
	"bufio"
	"context"
	"fmt"
	"io"
	mathRand "math/rand"
	"net"
	"net/http"
	"time"

	"github.com/komuw/goweb/cookie"
	"github.com/komuw/goweb/log"
)

const logIDKey = string(log.CtxKey)

// Log is a middleware that logs requests/responses.
func Log(wrappedHandler http.HandlerFunc, domain string, logOutput io.Writer) http.HandlerFunc {
	// We want different requests to share the same logger backed by the same circular buffere for storing logs.
	// That way, if someone makes one request and it succeds then they make another one that errors.
	// When the logs are been flushed for the request that errored, the logs for the request that succeeded will also be flushed.
	// Thus app developers can be able to correlate issues/logs in much better way.
	//
	// However, each request should get its own context. That's why we call `logger.WithCtx` for every request.
	mLogger := log.New(
		context.Background(),
		logOutput,
		// enought to hold messages for 5reqs/sec for 15minutes.
		5*60*15,
		// dont indent.
		false,
	)

	mathRand.Seed(time.Now().UTC().UnixNano())

	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := mLogger.WithCtx(ctx)

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
				// hopefully 15mins is enough.
				15*time.Minute,
				false,
			)
		}

		start := time.Now()
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
				"durationMS":  time.Since(start).Milliseconds(),
			}
			if gowebError := lrw.Header().Get(gowebMiddlewareErrorHeader); gowebError != "" {
				flds["gowebError"] = gowebError
			}
			lrw.Header().Del(gowebMiddlewareErrorHeader) // remove header so that users dont see it.

			if lrw.code == http.StatusServiceUnavailable || lrw.code == http.StatusTooManyRequests && w.Header().Get(retryAfterHeader) != "" {
				// We are either in load shedding or rate-limiting.
				// Only log 10% of the errors.
				shouldLog := mathRand.Intn(100) > 90
				if shouldLog {
					logger.Error(nil, flds)
				}
			} else if lrw.code >= http.StatusBadRequest {
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
}

var (
	// make sure we support http optional interfaces.
	// https://github.com/komuw/goweb/issues/15
	// https://blog.merovius.de/2017/07/30/the-trouble-with-optional-interfaces.html
	_ http.ResponseWriter = &logRW{}
	_ http.Flusher        = &logRW{}
	_ http.Hijacker       = &logRW{}
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
	return nil, nil, fmt.Errorf("http.Hijacker interface is not supported")
}

// getLogId returns a logID from the request or autogenerated if not available from the request.
func getLogId(req *http.Request) string {
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

	fromHeader := func(r *http.Request) string {
		if r != nil {
			if hdr := r.Header.Get(logIDKey); hdr != "" {
				return hdr
			}
		}
		return ""
	}

	fromCtx := func(ctx context.Context) string {
		return log.GetId(ctx)
	}

	// get logid in order of preference;
	// - cookie
	// - http headers
	// - context.Context

	if logID := fromCookie(req); logID != "" {
		return logID
	}

	if logID := fromHeader(req); logID != "" {
		return logID
	}

	return fromCtx(req.Context())
}
