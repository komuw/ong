package server

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/akshayjshah/attest"
	"github.com/komuw/goweb/middleware"
)

func TestDrainDuration(t *testing.T) {
	t.Parallel()

	t.Run("all in same units", func(t *testing.T) {
		t.Parallel()

		handlerTimeout := 170 * time.Second
		o := opts{
			port:              8080,
			host:              "127.0.0.1",
			network:           "tcp",
			readHeaderTimeout: 1 * time.Second,
			readTimeout:       1 * time.Second,
			writeTimeout:      160 * time.Second,
			handlerTimeout:    handlerTimeout,
			idleTimeout:       120 * time.Second,
		}
		got := drainDuration(o)
		want := handlerTimeout + (10 * time.Second)
		attest.Equal(t, got, want)
	})

	t.Run("different units", func(t *testing.T) {
		t.Parallel()

		writeTimeout := 3 * time.Minute
		o := opts{
			port:              8080,
			host:              "127.0.0.1",
			network:           "tcp",
			readHeaderTimeout: 1 * time.Nanosecond,
			readTimeout:       1 * time.Minute,
			writeTimeout:      writeTimeout,
			handlerTimeout:    170 * time.Millisecond,
			idleTimeout:       120 * time.Second,
		}
		got := drainDuration(o)
		want := writeTimeout + (10 * time.Second)
		attest.Equal(t, got, want)
	})
}

func TestOpts(t *testing.T) {
	t.Parallel()

	t.Run("sensible defaults", func(t *testing.T) {
		t.Parallel()

		got := DefaultOpts()
		want := opts{
			port:              8080,
			host:              "127.0.0.1",
			network:           "tcp",
			readHeaderTimeout: 1 * time.Second,
			readTimeout:       2 * time.Second,
			writeTimeout:      3 * time.Second,
			handlerTimeout:    13 * time.Second,
			idleTimeout:       113 * time.Second,
			serverPort:        ":8080",
			serverAddress:     "127.0.0.1:8080",
			httpPort:          ":8080",
		}
		attest.Equal(t, got, want)
	})

	t.Run("sensible defaults", func(t *testing.T) {
		t.Parallel()

		got := WithOpts(80, "localhost")
		want := opts{
			port:              80,
			host:              "localhost",
			network:           "tcp",
			readHeaderTimeout: 1 * time.Second,
			readTimeout:       2 * time.Second,
			writeTimeout:      3 * time.Second,
			handlerTimeout:    13 * time.Second,
			idleTimeout:       113 * time.Second,
			serverPort:        ":80",
			serverAddress:     "localhost:80",
			httpPort:          ":80",
		}
		attest.Equal(t, got, want)
	})
}

func someServerTestHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, msg)
	}
}

func TestServer(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		port := math.MaxUint16 - uint16(3)
		uri := "/api"
		msg := "hello world"
		mux := NewMux(
			Routes{
				NewRoute(
					uri,
					MethodGet,
					someServerTestHandler(msg),
					middleware.WithOpts("localhost"),
				),
			})

		go func() {
			err := Run(mux, WithOpts(port, "127.0.0.1"))
			attest.Ok(t, err)
		}()

		// await for the server to start.
		time.Sleep((1 * time.Second))

		res, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d%s", port, uri))
		attest.Ok(t, err)
		defer res.Body.Close()
		rb, err := io.ReadAll(res.Body)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)
	})
}
