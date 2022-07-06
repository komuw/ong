// Package middleware provides helpful functions that implement some common functionalities in http servers.
// A middleware is a func that returns a http.HandlerFunc
package middleware

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/akshayjshah/attest"
)

func someMiddlewareTestHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			b, e := io.ReadAll(r.Body)
			if e != nil {
				panic(e)
			}
			if len(b) > 1 {
				fmt.Fprint(w, string(b))
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

	msg := "hello world"
	errMsg := "not allowed. only allows http"
	tests := []struct {
		name               string
		middleware         func(wrappedHandler http.HandlerFunc, o Opts) http.HandlerFunc
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
		o := WithOpts("localhost", 443)
		wrappedHandler := All(someMiddlewareTestHandler(msg), o)
		ts := httptest.NewTLSServer(
			wrappedHandler,
		)
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

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			o := WithOpts("localhost", 443)
			wrappedHandler := tt.middleware(someMiddlewareTestHandler(msg), o)

			ts := httptest.NewTLSServer(
				wrappedHandler,
			)
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
			defer res.Body.Close()

			attest.Equal(t, res.StatusCode, tt.expectedStatusCode)
			attest.True(t, strings.Contains(string(rb), tt.expectedMsg))
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

	t.Run("integration with server succeds", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		o := WithOpts("localhost", 443)
		wrappedHandler := All(someMiddlewareTestHandler(msg), o)

		ts := httptest.NewTLSServer(
			wrappedHandler,
		)
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
			o := WithOpts("localhost", 443)
			msg := "hey"
			wrappedHandler := All(someMiddlewareTestHandler(msg), o)

			ts := httptest.NewTLSServer(
				wrappedHandler,
			)
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
		o := WithOpts("localhost", 443)
		wrappedHandler := All(someMiddlewareTestHandler(msg), o)

		ts := httptest.NewTLSServer(
			wrappedHandler,
		)
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

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		o := WithOpts("localhost", 443)
		// for this concurrency test, we have to re-use the same wrappedHandler
		// so that state is shared and thus we can see if there is any state which is not handled correctly.
		wrappedHandler := All(someMiddlewareTestHandler(msg), o)

		ts := httptest.NewTLSServer(
			wrappedHandler,
		)
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
	iterations := int(1.5 * defaultMinSize)
	msg := strings.Repeat("hello world", iterations)
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, msg)
	}
}

var resultBenchmarkAllMiddlewares int //nolint:gochecknoglobals

func BenchmarkAllMiddlewares(b *testing.B) {
	var r int
	o := WithOpts("localhost", 443)
	wrappedHandler := All(someBenchmarkAllMiddlewaresHandler(), o)
	ts := httptest.NewTLSServer(
		wrappedHandler,
	)
	defer ts.Close()

	tr := &http.Transport{
		// since we are using self-signed certificates, we need to skip verification.
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	intialRateLimiterSendRate := rateLimiterSendRate
	b.Cleanup(func() {
		rateLimiterSendRate = intialRateLimiterSendRate
	})
	// need to increase this  for tests otherwise the benchmark fails with http.StatusTooManyRequests
	rateLimiterSendRate = 500.0

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		// always record the result of Fib to prevent
		// the compiler eliminating the function call.
		req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
		attest.Ok(b, err)
		req.Header.Set(acceptEncodingHeader, "br;q=1.0, gzip;q=0.8, *;q=0.1")
		res, err := client.Do(req)
		attest.Ok(b, err)

		attest.Equal(b, res.StatusCode, http.StatusOK)
		attest.Equal(b, res.Header.Get(contentEncodingHeader), "gzip")
		r = res.StatusCode
	}
	// always store the result to a package level variable
	// so the compiler cannot eliminate the Benchmark itself.
	resultBenchmarkAllMiddlewares = r
}
