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
	// DefaultCsrfCookieMaxDuration is the duration that session cookie will be valid for by default.
	// [django] uses a value of 2 weeks by default.
	//
	// [django]: https://docs.djangoproject.com/en/4.1/ref/settings/#session-cookie-age
	DefaultSessionCookieMaxDuration = 14 * time.Hour
)

// session is a middleware that implements http sessions.
// It lets you store and retrieve arbitrary data on a per-site-visitor basis.
//
// This middleware works best when used together with the [sess] package.
func session(
	wrappedHandler http.Handler,
	secretKey string,
	domain string,
	sessionCookieMaxDuration time.Duration,
) http.HandlerFunc {
	if sessionCookieMaxDuration < 1*time.Second { // It is measured in seconds.
		sessionCookieMaxDuration = DefaultSessionCookieMaxDuration
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Read from cookies and check for session cookie.
		// 2. Get that cookie and save it to r.context
		r = sess.Initialise(r, secretKey)

		srw := newSessRW(w, r, domain, secretKey, sessionCookieMaxDuration)

		wrappedHandler.ServeHTTP(srw, r)
	}
}

// sessRW provides an http.ResponseWriter interface, which provides http session functionality.
type sessRW struct {
	http.ResponseWriter
	r                        *http.Request
	domain                   string
	secretKey                string
	sessionCookieMaxDuration time.Duration
	written                  bool
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
	// _ http.CloseNotifier  = &sessRW{} // `http.CloseNotifier` has been deprecated sinc Go v1.11(year 2018)
)

func newSessRW(
	w http.ResponseWriter,
	r *http.Request,
	domain string,
	secretKey string,
	sessionCookieMaxDuration time.Duration,
) *sessRW {
	return &sessRW{
		ResponseWriter:           w,
		r:                        r,
		domain:                   domain,
		secretKey:                secretKey,
		sessionCookieMaxDuration: sessionCookieMaxDuration,
		written:                  false,
	}
}

// Write saves session data.
func (srw *sessRW) Write(b []byte) (int, error) {
	// 3. Save session cookie to response.

	// We have to call `sess.Save` here.
	//
	// According to: https://pkg.go.dev/net/http#ResponseWriter
	// Changing the header map after a call to WriteHeader/Write has no effect unless in some specific cases.
	// Thus, we call sess.Save here just before any call to `ResponseWriter.Write` goes through.
	if !srw.written {
		sess.Save(
			srw.r,
			srw.ResponseWriter,
			srw.domain,
			srw.sessionCookieMaxDuration,
			srw.secretKey,
		)
		srw.written = true
	}

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
// https://github.com/caddyserver/caddy/blob/v2.7.4/modules/caddyhttp/responsewriter.go#L45-L49
func (srw *sessRW) ReadFrom(src io.Reader) (n int64, err error) {
	return io.Copy(srw.ResponseWriter, src)
}

// Unwrap implements http.ResponseController.
// It returns the underlying ResponseWriter,
// which is necessary for http.ResponseController to work correctly.
func (srw *sessRW) Unwrap() http.ResponseWriter {
	return srw.ResponseWriter
}
