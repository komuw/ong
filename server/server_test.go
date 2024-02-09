package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/komuw/ong/config"
	"github.com/komuw/ong/internal/tst"
	"github.com/komuw/ong/log"
	"github.com/komuw/ong/middleware"
	"github.com/komuw/ong/mux"

	"go.akshayshah.org/attest"
)

// todo: enable this.
//
// This is disabled because `server.Run()` starts a http server(aka goroutine)
// which is still running at the end of the test. Thus is technically a leak.
//
// func TestMain(m *testing.M) {
// 	// call flag.Parse() here if TestMain uses flags
// 	goleak.VerifyTestMain(m)
// }

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

	l := log.New(context.Background(), &bytes.Buffer{}, 500)

	t.Run("tls", func(t *testing.T) {
		t.Parallel()

		port := tst.GetPort()
		uri := "/api"
		msg := "hello world"
		mx := mux.New(
			config.WithOpts("localhost", port, tst.SecretKey(), config.DirectIpStrategy, l),
			nil,
			mux.NewRoute(
				uri,
				mux.MethodGet,
				someServerTestHandler(msg),
			),
		)

		go func() {
			err := Run(
				mx,
				config.WithOpts(
					"localhost",
					port,
					tst.SecretKey(),
					config.DirectIpStrategy,
					l,
				),
			)
			attest.Ok(t, err)
		}()

		// await for the server to start.
		attest.Ok(t, tst.Ping(port))

		{ // https server.
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

		{ // acme requests succeds.
			acmeUri := "/.well-known/acme-challenge/some-token"
			url := fmt.Sprintf(
				// note: acme request should be http(not https).
				//       it should also be to a domain(not an IP address).
				"http://localhost:%d%s",
				port-1,
				acmeUri,
			)
			res, err := client.Get(url)
			attest.Ok(t, err)

			defer res.Body.Close()
			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t,
				res.StatusCode,
				// Fails because the `acme.Handler()` will get called with a host like `localhost:38355`
				// and that host has no token configured for it.
				http.StatusInternalServerError,
			)
			attest.Subsequence(t, string(rb), "no such file or directory")
		}

		{ // redirect server
			res, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d%s", port-1, uri))
			attest.Ok(t, err)

			defer res.Body.Close()
			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), msg)
		}

		{ // http2.
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

		port := tst.GetPort()
		uri := "/api"
		msg := "hello world"
		mx := mux.New(
			config.WithOpts("localhost", port, tst.SecretKey(), config.DirectIpStrategy, l),
			nil,
			mux.NewRoute(
				uri,
				mux.MethodPost,
				someServerTestHandler(msg),
			),
		)

		go func() {
			err := Run(
				mx,
				config.WithOpts(
					"localhost",
					port,
					tst.SecretKey(),
					config.DirectIpStrategy,
					l,
				),
			)
			attest.Ok(t, err)
		}()

		// await for the server to start.
		attest.Ok(t, tst.Ping(port))

		t.Run("smallSize", func(t *testing.T) {
			postMsg := strings.Repeat("a", int(config.DefaultMaxBodyBytes/100))
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
			postMsg := strings.Repeat("a", int(config.DefaultMaxBodyBytes*2))
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

		port := tst.GetPort()
		uri := "/api"
		msg := "hello world"
		mx := mux.New(
			config.WithOpts("localhost", port, tst.SecretKey(), config.DirectIpStrategy, l),
			nil,
			mux.NewRoute(
				uri,
				mux.MethodGet,
				someServerTestHandler(msg),
			),
		)

		go func() {
			err := Run(
				mx,
				config.WithOpts(
					"localhost",
					port,
					tst.SecretKey(),
					config.DirectIpStrategy,
					l,
				),
			)
			attest.Ok(t, err)
		}()

		// await for the server to start.
		attest.Ok(t, tst.Ping(port))

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

	l := log.New(context.Background(), &bytes.Buffer{}, 500)

	handler := benchmarkServerHandler("helloWorld")
	port := tst.GetPort()

	go func() {
		time.Sleep(1 * time.Second)
		err := Run(
			handler,
			config.WithOpts(
				"localhost",
				port,
				tst.SecretKey(),
				config.DirectIpStrategy,
				l,
			),
		)
		attest.Ok(b, err)
	}()

	// await for the server to start.
	attest.Ok(b, tst.Ping(port))

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
			for range b.N {
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
