package middleware

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/komuw/ong/config"
	"github.com/komuw/ong/sess"
)

// session is a middleware that implements http sessions.
// It lets you store and retrieve arbitrary data on a per-site-visitor basis.
//
// This middleware works best when used together with the [sess] package.
func session(
	wrappedHandler http.Handler,
	secretKey string,
	sessionCookieDuration time.Duration,
	antiReplay func(r http.Request) string,
) http.HandlerFunc {
	if sessionCookieDuration < 1*time.Second { // It is measured in seconds.
		sessionCookieDuration = config.DefaultSessionCookieDuration
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Set anti replay data.
		// 2. Read from cookies and check for session cookie.
		// 3. Get that cookie and save it to r.context
		r = sess.Initialise(r, secretKey, antiReplay(*r))

		srw := newSessRW(w, r, secretKey, sessionCookieDuration)

		wrappedHandler.ServeHTTP(srw, r)

		// The handler may not have written anything.
		srw.save()
	}
}

// sessRW provides an http.ResponseWriter interface, which provides http session functionality.
type sessRW struct {
	http.ResponseWriter
	r                     *http.Request
	secretKey             string
	sessionCookieDuration time.Duration
	written               bool
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
	_ httpRespCtrler      = &sessRW{}
	// _ http.CloseNotifier  = &sessRW{} // `http.CloseNotifier` has been deprecated sinc Go v1.11(year 2018)
)

func newSessRW(
	w http.ResponseWriter,
	r *http.Request,
	secretKey string,
	sessionCookieDuration time.Duration,
) *sessRW {
	return &sessRW{
		ResponseWriter:        w,
		r:                     r,
		secretKey:             secretKey,
		sessionCookieDuration: sessionCookieDuration,
		written:               false,
	}
}

// save saves the session cookie to the response.
//
// According to: https://pkg.go.dev/net/http#ResponseWriter
// Changing the header map after a call to WriteHeader/Write has no effect unless in some specific cases.
// Thus, it has to be called before any method that can send the headers.
func (srw *sessRW) save() {
	if srw.written {
		return
	}
	srw.written = true
	sess.Save(
		srw.r,
		srw.ResponseWriter,
		srw.sessionCookieDuration,
		srw.secretKey,
	)
}

// Write saves session data.
func (srw *sessRW) Write(b []byte) (int, error) {
	srw.save()
	return srw.ResponseWriter.Write(b)
}

// WriteHeader saves session data.
func (srw *sessRW) WriteHeader(statusCode int) {
	srw.save()
	srw.ResponseWriter.WriteHeader(statusCode)
}

// Flush implements http.Flusher
func (srw *sessRW) Flush() {
	srw.save()
	if fw, ok := srw.ResponseWriter.(http.Flusher); ok {
		fw.Flush()
	}
}

// Hijack implements http.Hijacker
func (srw *sessRW) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := srw.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("ong/middleware/session: http.Hijacker interface is not supported")
}

// Push implements http.Pusher
func (srw *sessRW) Push(target string, opts *http.PushOptions) error {
	if p, ok := srw.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return fmt.Errorf("ong/middleware/session: http.Pusher interface is not supported")
}

// ReadFrom implements io.ReaderFrom
// It is necessary for the sendfile syscall
// https://github.com/caddyserver/caddy/pull/5022
// https://github.com/caddyserver/caddy/blob/v2.7.4/modules/caddyhttp/responsewriter.go#L45-L49
func (srw *sessRW) ReadFrom(src io.Reader) (n int64, err error) {
	srw.save()
	return io.Copy(srw.ResponseWriter, src)
}

// Unwrap implements http.ResponseController.
// It returns the underlying ResponseWriter,
// which is necessary for http.ResponseController to work correctly.
func (srw *sessRW) Unwrap() http.ResponseWriter {
	return srw.ResponseWriter
}
