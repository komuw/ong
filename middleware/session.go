package middleware

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/komuw/ong/sess"
)

const (
	// django uses a value of 2 weeks by default.
	// https://docs.djangoproject.com/en/4.1/ref/settings/#session-cookie-age
	sessionMaxAge = 14 * time.Hour
)

// Session is a middleware that implements http sessions.
// It lets you store and retrieve arbitrary data on a per-site-visitor basis.
//
// This middleware works best when used together with [ong/sess] package
//
// [ong/sess]: github.com/komuw/ong/sess
func Session(wrappedHandler http.HandlerFunc, secretKey, domain string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Read from cookies and check for session cookie.
		// 2. Get that cookie and save it to r.context
		r = sess.Initialise(r, secretKey)

		srw := newSessRW(w, r, domain, secretKey)

		wrappedHandler(srw, r)
	}
}

// sessRW provides an http.ResponseWriter interface, which provides session functionality.
type sessRW struct {
	http.ResponseWriter
	r         *http.Request
	domain    string
	secretKey string
}

var (
	// make sure we support http optional interfaces.
	// https://github.com/komuw/ong/issues/15
	// https://blog.merovius.de/2017/07/30/the-trouble-with-optional-interfaces.html
	_ http.ResponseWriter = &sessRW{}
	_ http.Flusher        = &sessRW{}
	_ http.Hijacker       = &sessRW{}
	_ http.Pusher         = &sessRW{}
	_ io.ReaderFrom       = &sessRW{}
	// _ http.CloseNotifier  = &logRW{} // `http.CloseNotifier` has been deprecated sinc Go v1.11(year 2018)
)

func newSessRW(
	w http.ResponseWriter,
	r *http.Request,
	domain string,
	secretKey string,
) *sessRW {
	return &sessRW{
		ResponseWriter: w,
		r:              r,
		domain:         domain,
		secretKey:      secretKey,
	}
}

// Write recodes the size of bytes sent for logging purposes.
func (srw *sessRW) Write(b []byte) (int, error) {
	// Save session cookie to response.
	sess.Save(
		srw.r,
		srw.ResponseWriter,
		srw.domain,
		sessionMaxAge,
		srw.secretKey,
	)

	return srw.ResponseWriter.Write(b)
}

// Flush implements http.Flusher
func (srw *sessRW) Flush() {
	if fw, ok := srw.ResponseWriter.(http.Flusher); ok {
		fw.Flush()
	}
}

// Hijack implements http.Hijacker
func (srw *sessRW) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := srw.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("ong/middleware: http.Hijacker interface is not supported")
}

// Push implements http.Pusher
func (srw *sessRW) Push(target string, opts *http.PushOptions) error {
	if p, ok := srw.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return fmt.Errorf("ong/middleware: http.Pusher interface is not supported")
}

// ReadFrom implements io.ReaderFrom
// It is necessary for the sendfile syscall
// https://github.com/caddyserver/caddy/pull/5022
func (srw *sessRW) ReadFrom(src io.Reader) (n int64, err error) {
	if rf, ok := srw.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(src)
	}
	return io.Copy(srw.ResponseWriter, src)
}
