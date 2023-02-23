package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/akshayjshah/attest"
	"github.com/komuw/ong/log"
	"github.com/komuw/ong/middleware"
	"github.com/komuw/ong/mux"
)

func getSecretKey() string {
	key := "hard-password"
	return key
}

func TestDrainDuration(t *testing.T) {
	t.Parallel()

	t.Run("all in same units", func(t *testing.T) {
		t.Parallel()

		handlerTimeout := 170 * time.Second
		o := Opts{
			port:              65080,
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
		o := Opts{
			port:              65080,
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

	{ // remove certs, so as to force new ones to get created.
		certPath, _ := certKeyPaths()
		os.RemoveAll(certPath)
	}

	t.Run("default opts", func(t *testing.T) {
		t.Parallel()

		l := log.New(&bytes.Buffer{}, 500)(context.Background())
		got := DevOpts(l)
		want := Opts{
			port:              65081,
			host:              "127.0.0.1",
			network:           "tcp",
			readHeaderTimeout: 1 * time.Second,
			readTimeout:       2 * time.Second,
			writeTimeout:      3 * time.Second,
			handlerTimeout:    13 * time.Second,
			idleTimeout:       113 * time.Second,
			serverPort:        ":65081",
			serverAddress:     "127.0.0.1:65081",
			httpPort:          ":65080",
			tls: tlsOpts{
				certFile: "/tmp/ong_dev_certificate.pem",
				keyFile:  "/tmp/ong_dev_key.pem",
				domain:   "localhost",
			},
		}
		attest.Equal(t, got, want)
	})

	t.Run("with opts", func(t *testing.T) {
		t.Parallel()

		certFile, keyFile := certKeyPaths()
		got := withOpts(80, certFile, keyFile, "", "*.example.com")

		want := Opts{
			port:              80,
			host:              "0.0.0.0",
			network:           "tcp",
			readHeaderTimeout: 1 * time.Second,
			readTimeout:       2 * time.Second,
			writeTimeout:      3 * time.Second,
			handlerTimeout:    13 * time.Second,
			idleTimeout:       113 * time.Second,
			serverPort:        ":80",
			serverAddress:     "0.0.0.0:80",
			httpPort:          ":79",
			tls: tlsOpts{
				certFile: "/tmp/ong_dev_certificate.pem",
				keyFile:  "/tmp/ong_dev_key.pem",
				domain:   "*.example.com",
			},
		}
		attest.Equal(t, got, want)
	})

	t.Run("default tls opts", func(t *testing.T) {
		t.Parallel()

		l := log.New(&bytes.Buffer{}, 500)(context.Background())
		got := DevOpts(l)
		want := Opts{
			port:              65081,
			host:              "127.0.0.1",
			network:           "tcp",
			readHeaderTimeout: 1 * time.Second,
			readTimeout:       2 * time.Second,
			writeTimeout:      3 * time.Second,
			handlerTimeout:    13 * time.Second,
			idleTimeout:       113 * time.Second,
			tls: tlsOpts{
				certFile: "/tmp/ong_dev_certificate.pem",
				keyFile:  "/tmp/ong_dev_key.pem",
				domain:   "localhost",
			},
			serverPort:    ":65081",
			serverAddress: "127.0.0.1:65081",
			httpPort:      ":65080",
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

	tr := &http.Transport{
		// since we are using self-signed certificates, we need to skip verification.
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	l := log.New(&bytes.Buffer{}, 500)(context.Background())

	t.Run("tls", func(t *testing.T) {
		t.Parallel()

		if os.Getenv("GITHUB_ACTIONS") != "" {
			// CreateDevCertKey() fails in github actions with error: `panic: open /home/runner/ong/rootCA_key.pem: permission denied`
			return
		}

		port := uint16(65081)
		uri := "/api"
		msg := "hello world"
		mux := mux.New(
			l,
			middleware.WithOpts("localhost", port, getSecretKey(), middleware.DirectIpStrategy, l),
			nil,
			mux.NewRoute(
				uri,
				mux.MethodGet,
				someServerTestHandler(msg),
			),
		)

		go func() {
			_, _ = CreateDevCertKey(l)
			time.Sleep(1 * time.Second)
			err := Run(mux, DevOpts(l), l)
			attest.Ok(t, err)
		}()

		// await for the server to start.
		time.Sleep(11 * time.Second)

		{
			// https server.
			res, err := client.Get(fmt.Sprintf(
				// note: the https scheme.
				"https://127.0.0.1:%d%s",
				port,
				uri,
			))
			attest.Ok(t, err)

			defer res.Body.Close()
			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), msg)
		}

		{
			// redirect server
			res, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d%s", port-1, uri))
			attest.Ok(t, err)

			defer res.Body.Close()
			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), msg)
		}

		{
			// http2.

			tr2 := &http.Transport{
				// since we are using self-signed certificates, we need to skip verification.
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				// using a non-zero `TLSClientConfig`(as above) disables http2.
				// so we have to force it.
				ForceAttemptHTTP2: true,
			}
			client2 := &http.Client{Transport: tr2}
			res, err := client2.Get(fmt.Sprintf(
				// note: the https scheme.
				"https://127.0.0.1:%d%s",
				port,
				uri,
			))
			attest.Ok(t, err)

			defer res.Body.Close()
			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), msg)
		}
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		if os.Getenv("GITHUB_ACTIONS") != "" {
			// CreateDevCertKey() fails in github actions with error: `panic: open /home/runner/ong/rootCA_key.pem: permission denied`
			return
		}

		port := math.MaxUint16 - uint16(3)
		uri := "/api"
		msg := "hello world"
		mux := mux.New(
			l,
			middleware.WithOpts("localhost", port, getSecretKey(), middleware.DirectIpStrategy, l),
			nil,
			mux.NewRoute(
				uri,
				mux.MethodGet,
				someServerTestHandler(msg),
			),
		)

		go func() {
			certFile, keyFile := CreateDevCertKey(l)
			err := Run(mux, withOpts(port, certFile, keyFile, "", "localhost"), l)
			attest.Ok(t, err)
		}()

		// await for the server to start.
		time.Sleep(11 * time.Second)

		runhandler := func() {
			res, err := client.Get(fmt.Sprintf("https://127.0.0.1:%d%s", port, uri)) // note: the https scheme.
			attest.Ok(t, err)

			defer res.Body.Close()
			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), msg)
		}

		wg := &sync.WaitGroup{}
		for rN := 0; rN <= 10; rN++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				runhandler()
			}()
		}
		wg.Wait()
	})
}
