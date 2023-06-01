package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/komuw/ong/log"
	"github.com/komuw/ong/middleware"
	"github.com/komuw/ong/mux"

	"go.akshayshah.org/attest"
)

func getSecretKey() string {
	key := "hard-password"
	return key
}

// getPort returns a random port.
// The idea is that different tests should run on different independent ports to avoid collisions.
func getPort() uint16 {
	r := rand.Intn(100) + 1
	p := math.MaxUint16 - uint16(r)
	return p
}

// todo: enable this.
//
// This is disabled because `server.Run()` starts a http server(aka goroutine)
// which is still running at the end of the test. Thus is technically a leak.
//
// func TestMain(m *testing.M) {
// 	// call flag.Parse() here if TestMain uses flags
// 	goleak.VerifyTestMain(m)
// }

func TestOpts(t *testing.T) {
	t.Parallel()

	t.Run("default opts", func(t *testing.T) {
		t.Parallel()

		l := log.New(&bytes.Buffer{}, 500)(context.Background())
		got := DevOpts(l)
		want := Opts{
			port:              65081,
			maxBodyBytes:      defaultMaxBodyBytes,
			host:              "127.0.0.1",
			network:           "tcp",
			readHeaderTimeout: 1 * time.Second,
			readTimeout:       2 * time.Second,
			writeTimeout:      3 * time.Second,
			handlerTimeout:    13 * time.Second,
			idleTimeout:       113 * time.Second,
			drainTimeout:      defaultDrainDuration,
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
			maxBodyBytes:      defaultMaxBodyBytes,
			host:              "0.0.0.0",
			network:           "tcp",
			readHeaderTimeout: 1 * time.Second,
			readTimeout:       2 * time.Second,
			writeTimeout:      3 * time.Second,
			handlerTimeout:    13 * time.Second,
			idleTimeout:       113 * time.Second,
			drainTimeout:      defaultDrainDuration,
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
			maxBodyBytes:      defaultMaxBodyBytes,
			host:              "127.0.0.1",
			network:           "tcp",
			readHeaderTimeout: 1 * time.Second,
			readTimeout:       2 * time.Second,
			writeTimeout:      3 * time.Second,
			handlerTimeout:    13 * time.Second,
			idleTimeout:       113 * time.Second,
			drainTimeout:      defaultDrainDuration,
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

const tlsFingerPrintKey = "TlsFingerPrintKey"

func someServerTestHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(tlsFingerPrintKey, middleware.ClientFingerPrint(r))

		if _, err := io.ReadAll(r.Body); err != nil {
			// This is where the error produced by `http.MaxBytesHandler` is produced at.
			// ie, its produced when a read is made.
			fmt.Fprint(w, err.Error())
			return
		}

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
	t.Cleanup(func() {
		// Without this, `uber/goleak` would report a leak.
		// see: https://github.com/uber-go/goleak/issues/87
		client.CloseIdleConnections()
	})

	l := log.New(&bytes.Buffer{}, 500)(context.Background())

	t.Run("tls", func(t *testing.T) {
		t.Parallel()

		port := getPort()
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
			certFile, keyFile := createDevCertKey(l)
			err := Run(mux, withOpts(port, certFile, keyFile, "", "localhost"), l)
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
			attest.NotZero(t, res.Header.Get(tlsFingerPrintKey))
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
			t.Cleanup(func() {
				// Without this, `uber/goleak` would report a leak.
				// see: https://github.com/uber-go/goleak/issues/87
				client2.CloseIdleConnections()
			})
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

	t.Run("request body size", func(t *testing.T) {
		t.Parallel()

		trReqBody := &http.Transport{
			// since we are using self-signed certificates, we need to skip verification.
			TLSClientConfig:    &tls.Config{InsecureSkipVerify: true},
			DisableCompression: true,
		}
		clientReqBody := &http.Client{Transport: trReqBody}
		t.Cleanup(func() {
			// Without this, `uber/goleak` would report a leak.
			// see: https://github.com/uber-go/goleak/issues/87
			clientReqBody.CloseIdleConnections()
		})

		port := getPort()
		uri := "/api"
		msg := "hello world"
		mux := mux.New(
			l,
			middleware.WithOpts("localhost", port, getSecretKey(), middleware.DirectIpStrategy, l),
			nil,
			mux.NewRoute(
				uri,
				mux.MethodPost,
				someServerTestHandler(msg),
			),
		)

		go func() {
			certFile, keyFile := createDevCertKey(l)
			err := Run(mux, withOpts(port, certFile, keyFile, "", "localhost"), l)
			attest.Ok(t, err)
		}()

		// await for the server to start.
		time.Sleep(11 * time.Second)

		t.Run("smallSize", func(t *testing.T) {
			postMsg := strings.Repeat("a", int(defaultMaxBodyBytes/100))
			body := strings.NewReader(postMsg)
			url := fmt.Sprintf("https://127.0.0.1:%d%s", port, uri)
			res, err := clientReqBody.Post(url, "text/plain", body)
			attest.Ok(t, err)

			defer res.Body.Close()
			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), msg)
		})

		t.Run("largeSize", func(t *testing.T) {
			postMsg := strings.Repeat("a", int(defaultMaxBodyBytes*2))
			body := strings.NewReader(postMsg)

			url := fmt.Sprintf("https://127.0.0.1:%d%s", port, uri)
			req, err := http.NewRequest("POST", url, body)
			attest.Ok(t, err)
			req.ContentLength = int64(body.Len())
			req.Header.Set("Content-Type", "text/plain")
			req.Header.Set("Accept-Encoding", "identity")

			res, err := clientReqBody.Do(req)
			attest.Ok(t, err)

			defer res.Body.Close()
			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t, res.StatusCode, http.StatusOK)
			// The error message is guaranteed by Go's compatibility promise.
			// see: https://github.com/golang/go/blob/go1.20.3/src/net/http/request.go#L1153-L1156
			errMsg := "request body too large"
			attest.Subsequence(t, string(rb), errMsg)
		})
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		port := getPort()
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
			certFile, keyFile := createDevCertKey(l)
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

func benchmarkServerHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, msg)
	}
}

var result int //nolint:gochecknoglobals

func BenchmarkServer(b *testing.B) {
	// This benchmarks the server end-to-end.
	// For example we can use it to see the effect of tls fingerprinting on requests throughput/latency.
	//

	l := log.New(&bytes.Buffer{}, 500)(context.Background())

	handler := benchmarkServerHandler("helloWorld")
	port := getPort()

	go func() {
		certFile, keyFile := createDevCertKey(l)
		time.Sleep(1 * time.Second)
		err := Run(handler, withOpts(port, certFile, keyFile, "", "localhost"), l)
		attest.Ok(b, err)
	}()

	// await for the server to start.
	time.Sleep(11 * time.Second)

	b.ResetTimer()
	b.ReportAllocs()

	for _, disableKeepAlive := range [2]bool{true, false} {
		b.Run(fmt.Sprintf("DisableKeepAlives: %v", disableKeepAlive), func(b *testing.B) {
			tr := &http.Transport{
				// see: http.DefaultTransport
				DisableKeepAlives: disableKeepAlive,

				// since we are using self-signed certificates, we need to skip verification.
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				// using a non-zero `TLSClientConfig`(as above) disables http2.
				// so we have to force it.
				ForceAttemptHTTP2: true,
			}
			c := &http.Client{Transport: tr}
			url := fmt.Sprintf("https://localhost:%d", port)
			var count int64 = 0
			var r int

			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				// The loop body is executed b.N times total across all goroutines.
				res, err := c.Get(url)
				attest.Ok(b, err)

				_, _ = io.Copy(io.Discard, res.Body)
				res.Body.Close()

				attest.Equal(b, res.StatusCode, http.StatusOK)
				atomic.AddInt64(&count, 1)
				r = res.StatusCode
			}
			b.ReportMetric(float64(count)/b.Elapsed().Seconds(), "req/s")

			// always store the result to a package level variable
			// so the compiler cannot eliminate the Benchmark itself.
			result = r
		})
	}
}
