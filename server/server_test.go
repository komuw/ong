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
		rc := runContext{
			port:              "8080",
			network:           "tcp",
			host:              "127.0.0.1",
			handlerTimeout:    handlerTimeout,
			readHeaderTimeout: 1 * time.Second,
			readTimeout:       1 * time.Second,
			writeTimeout:      160 * time.Second,
			idleTimeout:       120 * time.Second,
		}
		got := drainDuration(rc)
		want := handlerTimeout + (10 * time.Second)
		attest.Equal(t, got, want)
	})

	t.Run("different units", func(t *testing.T) {
		t.Parallel()

		writeTimeout := 3 * time.Minute
		rc := runContext{
			port:              "8080",
			network:           "tcp",
			host:              "127.0.0.1",
			handlerTimeout:    170 * time.Millisecond,
			readHeaderTimeout: 1 * time.Nanosecond,
			readTimeout:       1 * time.Minute,
			writeTimeout:      writeTimeout,
			idleTimeout:       120 * time.Second,
		}
		got := drainDuration(rc)
		want := writeTimeout + (10 * time.Second)
		attest.Equal(t, got, want)
	})
}
