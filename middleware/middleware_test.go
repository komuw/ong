package middleware

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/komuw/ong/config"
	"github.com/komuw/ong/id"
	"github.com/komuw/ong/internal/tst"
	"github.com/komuw/ong/log"

	"go.akshayshah.org/attest"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	// call flag.Parse() here if TestMain uses flags
	goleak.VerifyTestMain(m)
}

func someMiddlewareTestHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			b, e := io.ReadAll(r.Body)
			if e != nil {
				panic(e)
			}
			if len(b) > 1 {
				_, _ = w.Write(b)
				return
			}
		}

		fmt.Fprint(w, msg)
	}
}

func TestAllMiddleware(t *testing.T) {
	t.Parallel()

	tr := &http.Transport{
		// since we are using self-signed certificates, we need to skip verification.
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	l := log.New(context.Background(), &bytes.Buffer{}, 500)

	msg := "hello world"
	errMsg := "not allowed. only allows http"
	tests := []struct {
		name               string
		middleware         func(wrappedHandler http.Handler, o config.Opts) http.HandlerFunc
		httpMethod         string
		expectedStatusCode int
		expectedMsg        string
	}{
		// All
		{
			name:               "All middleware http GET",
			middleware:         All,
			httpMethod:         http.MethodGet,
			expectedStatusCode: http.StatusOK,
			expectedMsg:        msg,
		},
		{
			name:               "All middleware http TRACE",
			middleware:         All,
			httpMethod:         http.MethodTrace,
			expectedStatusCode: http.StatusOK,
			expectedMsg:        msg,
		},

		// Get
		{
			name:               "Get middleware http GET",
			middleware:         Get,
			httpMethod:         http.MethodGet,
			expectedStatusCode: http.StatusOK,
			expectedMsg:        msg,
		},
		{
			name:               "Get middleware http TRACE",
			middleware:         Get,
			httpMethod:         http.MethodTrace,
			expectedStatusCode: http.StatusMethodNotAllowed,
			expectedMsg:        errMsg,
		},

		// Post
		{
			name:               "Post middleware http POST",
			middleware:         Post,
			httpMethod:         http.MethodPost,
			expectedStatusCode: http.StatusOK,
			expectedMsg:        msg,
		},
		{
			name:               "Post middleware http TRACE",
			middleware:         Post,
			httpMethod:         http.MethodTrace,
			expectedStatusCode: http.StatusMethodNotAllowed,
			expectedMsg:        errMsg,
		},

		// Head
		{
			name:               "Head middleware http HEAD",
			middleware:         Head,
			httpMethod:         http.MethodHead,
			expectedStatusCode: http.StatusOK,
			expectedMsg:        "", // the golang http-client does not return the body for HEAD requests.
		},
		{
			name:               "Head middleware http TRACE",
			middleware:         Head,
			httpMethod:         http.MethodTrace,
			expectedStatusCode: http.StatusMethodNotAllowed,
			expectedMsg:        errMsg,
		},

		// Put
		{
			name:               "Put middleware http PUT",
			middleware:         Put,
			httpMethod:         http.MethodPut,
			expectedStatusCode: http.StatusOK,
			expectedMsg:        msg,
		},
		{
			name:               "Put middleware http TRACE",
			middleware:         Put,
			httpMethod:         http.MethodTrace,
			expectedStatusCode: http.StatusMethodNotAllowed,
			expectedMsg:        errMsg,
		},

		// Delete
		{
			name:               "Delete middleware http DELETE",
			middleware:         Delete,
			httpMethod:         http.MethodDelete,
			expectedStatusCode: http.StatusOK,
			expectedMsg:        msg,
		},
		{
			name:               "Delete middleware http TRACE",
			middleware:         Delete,
			httpMethod:         http.MethodTrace,
			expectedStatusCode: http.StatusMethodNotAllowed,
			expectedMsg:        errMsg,
		},
	}

	csrfToken := ""
	{
		// non-safe http methods(like POST) require a server-known csrf token;
		// otherwise it fails with http 403
		// so here we make a http GET so that we can have a csrf token.
		httpsPort := tst.GetPort()
		domain := "localhost"
		o := config.WithOpts(domain, httpsPort, tst.SecretKey(), config.DirectIpStrategy, l)
		wrappedHandler := All(someMiddlewareTestHandler(msg), o)
		ts, err := tst.TlsServer(wrappedHandler, domain, httpsPort)
		attest.Ok(t, err)
		defer ts.Close()

		res, err := client.Get(ts.URL)
		attest.Ok(t, err)

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)
		t.Cleanup(func() {
			res.Body.Close()
		})

		csrfToken = res.Header.Get(CsrfHeader)
		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.NotZero(t, csrfToken)
		attest.Equal(t, string(rb), msg)
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			httpsPort := tst.GetPort()
			domain := "localhost"
			o := config.WithOpts(domain, httpsPort, tst.SecretKey(), config.DirectIpStrategy, l)
			wrappedHandler := tt.middleware(someMiddlewareTestHandler(msg), o)

			ts, err := tst.TlsServer(wrappedHandler, "localhost", httpsPort)
			attest.Ok(t, err)
			defer ts.Close()

			req, err := http.NewRequest(tt.httpMethod, ts.URL, nil)
			attest.Ok(t, err)
			req.AddCookie(
				&http.Cookie{
					Name:   csrfCookieName,
					Value:  csrfToken,
					Domain: "localhost",
				},
			)
			req.Header.Set(CsrfHeader, csrfToken) // setting the cookie appears not to work, so set the header.
			res, err := client.Do(req)
			attest.Ok(t, err)

			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)
			t.Cleanup(func() {
				res.Body.Close()
			})

			attest.Equal(t, res.StatusCode, tt.expectedStatusCode)
			attest.Subsequence(t, string(rb), tt.expectedMsg)
		})
	}
}

func TestMiddlewareServer(t *testing.T) {
	t.Parallel()

	tr := &http.Transport{
		// since we are using self-signed certificates, we need to skip verification.
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	l := log.New(context.Background(), &bytes.Buffer{}, 500)

	t.Run("integration with server succeds", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		httpsPort := tst.GetPort()
		domain := "localhost"
		o := config.WithOpts(domain, httpsPort, tst.SecretKey(), config.DirectIpStrategy, l)
		wrappedHandler := All(someMiddlewareTestHandler(msg), o)

		ts, err := tst.TlsServer(wrappedHandler, domain, httpsPort)
		attest.Ok(t, err)
		defer ts.Close()

		res, err := client.Get(ts.URL)
		attest.Ok(t, err)

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)
	})

	t.Run("http POST succeds", func(t *testing.T) {
		t.Parallel()

		csrfToken := ""
		{
			// non-safe http methods(like POST) require a server-known csrf token;
			// otherwise it fails with http 403
			// so here we make a http GET so that we can have a csrf token.
			httpsPort := tst.GetPort()
			domain := "localhost"
			o := config.WithOpts(domain, httpsPort, tst.SecretKey(), config.DirectIpStrategy, l)
			msg := "hey"
			wrappedHandler := All(someMiddlewareTestHandler(msg), o)

			ts, err := tst.TlsServer(wrappedHandler, domain, httpsPort)
			attest.Ok(t, err)
			defer ts.Close()

			res, err := client.Get(ts.URL)
			attest.Ok(t, err)

			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)
			defer res.Body.Close()

			csrfToken = res.Header.Get(CsrfHeader)
			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.NotZero(t, csrfToken)
			attest.Equal(t, string(rb), msg)
		}

		msg := "hello world"
		httpsPort := tst.GetPort()
		domain := "localhost"
		o := config.WithOpts(domain, httpsPort, tst.SecretKey(), config.DirectIpStrategy, l)
		wrappedHandler := All(someMiddlewareTestHandler(msg), o)

		ts, err := tst.TlsServer(wrappedHandler, domain, httpsPort)
		attest.Ok(t, err)
		defer ts.Close()

		postMsg := "This is a post message"
		req, err := http.NewRequest(http.MethodPost, ts.URL, strings.NewReader(postMsg))
		attest.Ok(t, err)
		req.Header.Set(CsrfHeader, csrfToken)
		res, err := client.Do(req)
		attest.Ok(t, err)

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), postMsg)
	})

	t.Run("acme succeds", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		domain := "localhost"
		o := config.WithOpts(domain, 443, tst.SecretKey(), config.DirectIpStrategy, l)
		wrappedHandler := All(someMiddlewareTestHandler(msg), o)

		// Should not be a `NewTLSServer` since acme requires HTTP(not HTTPS)
		ts := httptest.NewServer(
			wrappedHandler,
		)
		defer ts.Close()

		const acmeChallengeURI = "/.well-known/acme-challenge/"
		acmeChallengeURL := ts.URL + acmeChallengeURI
		acmeChallengeURL = strings.ReplaceAll(acmeChallengeURL, "127.0.0.1", domain)
		res, err := client.Get(acmeChallengeURL)
		attest.Ok(t, err)

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode,
			// Fails because the `acme.Handler()` will get called with a host like `localhost:38355`
			// and that host has no token configured for it.
			http.StatusInternalServerError)
		attest.Subsequence(t, string(rb), "no such file or directory")
	})

	t.Run("wildcard domain", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		httpsPort := tst.GetPort()
		domain := "*.localhost"
		o := config.WithOpts(domain, httpsPort, tst.SecretKey(), config.DirectIpStrategy, l)
		wrappedHandler := All(someMiddlewareTestHandler(msg), o)

		ts, err := tst.TlsServer(wrappedHandler, "localhost", httpsPort)
		attest.Ok(t, err)
		defer ts.Close()

		res, err := client.Get(ts.URL)
		attest.Ok(t, err)

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)
	})

	t.Run("double WriteHeader call", func(t *testing.T) {
		t.Parallel()
		// Note: Running this test will always produce the error message:
		//   `http: superfluous response.WriteHeader call from github.com/komuw/ong/middleware.(*logRW).WriteHeader (log.go:121)`
		// And the stack trace always points to the ong log middleware.
		// But that does not mean that ong has any issues. That error is always produced when
		// `http.ResponseWriter.WriteHeader()` is called more than once.
		// See: https://github.com/komuw/ong/issues/48#issuecomment-1260654535

		getLogger := func(w io.Writer) *slog.Logger {
			return log.New(context.Background(), w, 500)
		}
		logOutput := &bytes.Buffer{}
		msg := "hello"
		code := http.StatusAccepted
		httpsPort := tst.GetPort()
		domain := "*.localhost"
		o := config.WithOpts(domain, httpsPort, tst.SecretKey(), config.DirectIpStrategy, getLogger(logOutput))
		doubleWrite := func(msg string, code int) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
				w.WriteHeader(code) // causes the issue: https://github.com/komuw/ong/issues/48
				fmt.Fprint(w, msg)
			}
		}
		wrappedHandler := All(doubleWrite(msg, code), o)
		ts, err := tst.TlsServer(wrappedHandler, "localhost", httpsPort)
		attest.Ok(t, err)
		defer ts.Close()

		req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
		attest.Ok(t, err)
		req.AddCookie(&http.Cookie{
			Name:  logIDKey,
			Value: "hey-some-log-id:" + id.New(),
		})

		res, err := client.Do(req)
		attest.Ok(t, err)

		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, code)
		attest.Equal(t, string(rb), msg)
		attest.Zero(t, logOutput.String())
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		httpsPort := tst.GetPort()
		domain := "localhost"
		o := config.WithOpts(domain, httpsPort, tst.SecretKey(), config.DirectIpStrategy, l)
		// for this concurrency test, we have to re-use the same wrappedHandler
		// so that state is shared and thus we can see if there is any state which is not handled correctly.
		wrappedHandler := All(someMiddlewareTestHandler(msg), o)

		ts, errTls := tst.TlsServer(wrappedHandler, domain, httpsPort)
		attest.Ok(t, errTls)
		defer ts.Close()

		runhandler := func() {
			res, err := client.Get(ts.URL)
			attest.Ok(t, err)

			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)
			defer res.Body.Close()

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

func someBenchmarkAllMiddlewaresHandler() http.HandlerFunc {
	// bound stack growth.
	// see: https://github.com/komuw/ong/issues/54
	msg := strings.Repeat("hello world", 2)
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, msg)
	}
}

var resultBenchmarkAllMiddlewares int //nolint:gochecknoglobals

func BenchmarkAllMiddlewares(b *testing.B) {
	var r int
	l := log.New(context.Background(), &bytes.Buffer{}, 500)
	httpsPort := tst.GetPort()
	domain := "localhost"
	// need to increase rateLimit for tests otherwise the benchmark fails with http.StatusTooManyRequests
	rateLimit := 500.0
	o := config.New(
		domain,
		httpsPort,
		tst.SecretKey(),
		config.DirectIpStrategy,
		l,
		rateLimit,
		config.DefaultLoadShedSamplingPeriod,
		config.DefaultLoadShedMinSampleSize,
		config.DefaultLoadShedBreachLatency,
		nil,
		nil,
		nil,
		false,
		config.DefaultCorsCacheDuration,
		config.DefaultCsrfCookieDuration,
		config.DefaultSessionCookieDuration,
		config.DefaultSessionAntiReplayFunc,
		20*1024*1024,
		slog.LevelDebug,
		1*time.Second,
		1*time.Second,
		1*time.Second,
		1*time.Second,
		10*time.Second,
		"",
		"",
		"acme@example.org",
		[]string{domain},
		"",
		nil,
	)

	wrappedHandler := All(someBenchmarkAllMiddlewaresHandler(), o)

	ts, errTls := tst.TlsServer(wrappedHandler, domain, httpsPort)
	attest.Ok(b, errTls)
	defer ts.Close()

	tr := &http.Transport{
		// since we are using self-signed certificates, we need to skip verification.
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
		attest.Ok(b, err)
		req.Header.Set(acceptEncodingHeader, "br;q=1.0, gzip;q=0.8, *;q=0.1")
		res, err := client.Do(req)
		attest.Ok(b, err)
		defer res.Body.Close()

		attest.Equal(b, res.StatusCode, http.StatusOK)
		attest.Zero(b, res.Header.Get(contentEncodingHeader))
		r = res.StatusCode
	}
	// always store the result to a package level variable
	// so the compiler cannot eliminate the Benchmark itself.
	resultBenchmarkAllMiddlewares = r
}
