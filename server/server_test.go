package server

import (
	"testing"
	"time"

	"github.com/akshayjshah/attest"
)

func TestDrainDuration(t *testing.T) {
	t.Parallel()

	t.Run("all in same units", func(t *testing.T) {
		t.Parallel()

		handlerTimeout := 170 * time.Second
		rc := opts{
			port:              "8080",
			host:              "127.0.0.1",
			network:           "tcp",
			readHeaderTimeout: 1 * time.Second,
			readTimeout:       1 * time.Second,
			writeTimeout:      160 * time.Second,
			handlerTimeout:    handlerTimeout,
			idleTimeout:       120 * time.Second,
		}
		got := drainDuration(rc)
		want := handlerTimeout + (10 * time.Second)
		attest.Equal(t, got, want)
	})

	t.Run("different units", func(t *testing.T) {
		t.Parallel()

		writeTimeout := 3 * time.Minute
		rc := opts{
			port:              "8080",
			host:              "127.0.0.1",
			network:           "tcp",
			readHeaderTimeout: 1 * time.Nanosecond,
			readTimeout:       1 * time.Minute,
			writeTimeout:      writeTimeout,
			handlerTimeout:    170 * time.Millisecond,
			idleTimeout:       120 * time.Second,
		}
		got := drainDuration(rc)
		want := writeTimeout + (10 * time.Second)
		attest.Equal(t, got, want)
	})
}

func TestOpts(t *testing.T) {
	t.Run("sensible defaults", func(t *testing.T) {
		got := DefaultOpts()
		want := opts{
			port:              "8080",
			host:              "127.0.0.1",
			network:           "tcp",
			readHeaderTimeout: 1 * time.Second,
			readTimeout:       2 * time.Second,
			writeTimeout:      3 * time.Second,
			handlerTimeout:    13 * time.Second,
			idleTimeout:       113 * time.Second,
		}
		attest.Equal(t, got, want)
	})

	t.Run("sensible defaults", func(t *testing.T) {
		got := WithOpts("80", "localhost")
		want := opts{
			port:              "80",
			host:              "localhost",
			network:           "tcp",
			readHeaderTimeout: 1 * time.Second,
			readTimeout:       2 * time.Second,
			writeTimeout:      3 * time.Second,
			handlerTimeout:    13 * time.Second,
			idleTimeout:       113 * time.Second,
		}
		attest.Equal(t, got, want)
	})
}

// type myEH struct{ router *http.ServeMux }

// func (m *myEH) ServeHTTP(w http.ResponseWriter, r *http.Request) {
// 	m.router.ServeHTTP(w, r)
// }

// func (m *myEH) GetLogger() *log.Logger {
// 	return log.New(os.Stderr, "logger: ", log.Lshortfile)
// }

// func (m *myEH) Routes() {
// 	m.router.HandleFunc("/hello",
// 		echoHandler("hello"),
// 	)
// }

// // echoHandler echos back in the response, the msg that was passed in.
// func echoHandler(msg string) http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		fmt.Fprint(w, msg)
// 	}
// }

// func TestRun(t *testing.T) {
// 	t.Run("success", func(t *testing.T) {
// 		eh := &myEH{router: http.NewServeMux()}
// 		err := Run(eh, WithOpts("0", "localhost"))
// 		attest.Ok(t, err)
// 	})
// }
