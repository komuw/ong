package mux

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/akshayjshah/attest"
	"github.com/komuw/ong/log"
	"github.com/komuw/ong/middleware"
)

func getSecretKey() string {
	key := "hard-password"
	return key
}

func someMuxHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, msg)
	}
}

func thisIsAnitherMuxHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "thisIsAnitherMuxHandler")
	}
}

func TestNewRoute(t *testing.T) {
	t.Parallel()

	// l := log.New(&bytes.Buffer{}, 500)

	// succeds
	_ = NewRoute(
		"/api",
		MethodGet,
		someMuxHandler("msg"),
	)

	// todo: restore this logic.
	// Right now the logic is disabled because of `BasicAuth`
	//
	// fails
	// attest.Panics(t, func() {
	// 	_ = NewRoute(
	// 		"/api",
	// 		MethodGet,
	// 		middleware.Get(
	// 			someMuxHandler("msg"),
	// 			middleware.WithOpts("localhost", 443, getSecretKey(), l),
	// 		),
	// 	)
	// })
}

func TestMux(t *testing.T) {
	t.Parallel()

	tr := &http.Transport{
		// since we are using self-signed certificates, we need to skip verification.
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	l := log.New(&bytes.Buffer{}, 500)

	t.Run("unknown uri", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		mux := New(
			l,
			middleware.WithOpts("localhost", 443, getSecretKey(), l),
			nil,
			NewRoute(
				"/api",
				MethodGet,
				someMuxHandler(msg),
			),
		)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/UnknownUri", nil)
		mux.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusNotFound)
	})

	t.Run("unknown http method", func(t *testing.T) {
		t.Parallel()

		uri := "/api/" // forward slash at suffix is important.
		msg := "hello world"
		mux := New(
			l,
			middleware.WithOpts("localhost", 443, getSecretKey(), l),
			nil,
			NewRoute(
				uri,
				MethodGet,
				someMuxHandler(msg),
			),
		)

		ts := httptest.NewTLSServer(
			mux,
		)
		defer ts.Close()

		csrfToken := ""
		{
			// non-safe http methods(like POST) require a server-known csrf token;
			// otherwise it fails with http 403
			// so here we make a http GET so that we can have a csrf token.
			res, err := client.Get(ts.URL + uri)
			attest.Ok(t, err)
			defer res.Body.Close()

			csrfToken = res.Header.Get(middleware.CsrfHeader)
			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.NotZero(t, csrfToken)
		}

		req, err := http.NewRequest(http.MethodPost, ts.URL+uri, nil)
		attest.Ok(t, err)
		req.Header.Set(middleware.CsrfHeader, csrfToken)
		res, err := client.Do(req)
		attest.Ok(t, err)
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusMethodNotAllowed)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		uri := "/api"
		mux := New(
			l,
			middleware.WithOpts("localhost", 443, getSecretKey(), l),
			nil,
			NewRoute(
				uri,
				MethodGet,
				someMuxHandler(msg),
			),
		)

		ts := httptest.NewTLSServer(
			mux,
		)
		defer ts.Close()

		res, err := client.Get(ts.URL + uri)
		attest.Ok(t, err)

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)
	})

	t.Run("conflict detected", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		uri1 := "/api/hi"
		uri2 := "/api/:someId"
		method := MethodGet

		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("expected a panic, yet did not panic.")
			}

			rStr := fmt.Sprintf("%v", r)
			attest.Subsequence(t, rStr, uri2)
			attest.Subsequence(t, rStr, method)
			attest.Subsequence(t, rStr, "ong/mux/mux_test.go:23") // location where `someMuxHandler` is declared.
			attest.Subsequence(t, rStr, "ong/mux/mux_test.go:29") // location where `thisIsAnitherMuxHandler` is declared.
		}()

		_ = New(
			l,
			middleware.WithOpts("localhost", 443, getSecretKey(), l),
			nil,
			NewRoute(
				uri1,
				method,
				someMuxHandler(msg),
			),
			NewRoute(
				uri2,
				method,
				thisIsAnitherMuxHandler(),
			),
		)
	})
}

func getManyRoutes() []Route {
	routes := []Route{}

	for i := 0; i <= 200; i++ {
		uri := fmt.Sprintf("uri-%d", i)
		routes = append(
			routes,
			NewRoute(
				uri,
				MethodAll,
				someMuxHandler(uri),
			),
		)
	}

	return routes
}

var result Mux //nolint:gochecknoglobals

func BenchmarkMuxNew(b *testing.B) {
	var r Mux

	l := log.New(&bytes.Buffer{}, 500)

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		mux := New(
			l,
			middleware.WithOpts("localhost", 443, getSecretKey(), l),
			nil,
			getManyRoutes()...,
		)
		r = mux
	}
	// always store the result to a package level variable
	// so the compiler cannot eliminate the Benchmark itself.
	result = r
}
