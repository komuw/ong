package mux

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/komuw/ong/internal/tst"
	"github.com/komuw/ong/log"
	"github.com/komuw/ong/middleware"

	"go.akshayshah.org/attest"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	// call flag.Parse() here if TestMain uses flags
	goleak.VerifyTestMain(m)
}

func someMuxHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, msg)
	}
}

func thisIsAnotherMuxHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "thisIsAnotherMuxHandler")
	}
}

func checkAgeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		age := Param(r.Context(), "age")
		_, _ = fmt.Fprintf(w, "Age is %s", age)
	}
}

func TestNewRoute(t *testing.T) {
	t.Parallel()

	l := log.New(context.Background(), &bytes.Buffer{}, 500)

	// succeds
	_ = NewRoute(
		"/api",
		MethodGet,
		someMuxHandler("msg"),
	)

	// succeds
	_ = NewRoute(
		"/api",
		MethodGet,
		middleware.BasicAuth(someMuxHandler("msg"), "some-user", "some-very-very-h1rd-passwd"),
	)

	// fails
	attest.Panics(t, func() {
		_ = NewRoute(
			"/api",
			MethodGet,
			middleware.Get(
				someMuxHandler("msg"),
				middleware.WithOpts("localhost", 443, tst.SecretKey(), middleware.DirectIpStrategy, l),
			),
		)
	})
}

func TestMux(t *testing.T) {
	t.Parallel()

	tr := &http.Transport{
		// since we are using self-signed certificates, we need to skip verification.
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	l := log.New(context.Background(), &bytes.Buffer{}, 500)

	t.Run("unknown uri", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		mux := New(
			middleware.WithOpts("localhost", 443, tst.SecretKey(), middleware.DirectIpStrategy, l),
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
		httpsPort := tst.GetPort()
		domain := "localhost"
		mux := New(
			middleware.WithOpts(domain, httpsPort, tst.SecretKey(), middleware.DirectIpStrategy, l),
			nil,
			NewRoute(
				uri,
				MethodGet,
				someMuxHandler(msg),
			),
		)

		ts, errTlS := tst.TlsServer(mux, domain, httpsPort)
		attest.Ok(t, errTlS)
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
		httpsPort := tst.GetPort()
		domain := "localhost"
		mux := New(
			middleware.WithOpts(domain, httpsPort, tst.SecretKey(), middleware.DirectIpStrategy, l),
			nil,
			NewRoute(
				uri,
				MethodGet,
				someMuxHandler(msg),
			),
		)

		ts, err := tst.TlsServer(mux, domain, httpsPort)
		attest.Ok(t, err)
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
			attest.Subsequence(t, rStr, "ong/mux/mux_test.go:27") // location where `someMuxHandler` is declared.
			attest.Subsequence(t, rStr, "ong/mux/mux_test.go:33") // location where `thisIsAnotherMuxHandler` is declared.
		}()

		_ = New(
			middleware.WithOpts("localhost", 443, tst.SecretKey(), middleware.DirectIpStrategy, l),
			nil,
			NewRoute(
				uri1,
				method,
				someMuxHandler(msg),
			),
			NewRoute(
				uri2,
				method,
				thisIsAnotherMuxHandler(),
			),
		)
	})

	t.Run("resolve url", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		expectedHandler := someMuxHandler(msg)
		mux := New(
			middleware.WithOpts("localhost", 443, tst.SecretKey(), middleware.DirectIpStrategy, l),
			nil,
			NewRoute(
				"/api",
				MethodGet,
				expectedHandler,
			),
			NewRoute(
				"check/:age/",
				MethodAll,
				checkAgeHandler(),
			),
		)

		tests := []struct {
			name      string
			path      string
			pattern   string
			method    string
			stackPath string
		}{
			{
				"success with no slashes",
				"api",
				"/api/",
				MethodGet,
				"ong/mux/mux_test.go:27", // location where `someMuxHandler` is declared.
			},
			{
				"success with prefix slash",
				"/api",
				"/api/",
				MethodGet,
				"ong/mux/mux_test.go:27", // location where `someMuxHandler` is declared.
			},
			{
				"success with suffix slash",
				"api/",
				"/api/",
				MethodGet,
				"ong/mux/mux_test.go:27", // location where `someMuxHandler` is declared.
			},
			{
				"success with all slashes",
				"/api/",
				"/api/",
				MethodGet,
				"ong/mux/mux_test.go:27", // location where `someMuxHandler` is declared.
			},
			{
				"bad",
				"/",
				"",
				"",
				"",
			},
			{
				"url with param",
				"check/2625",
				"/check/:age/",
				MethodAll,
				"ong/mux/mux_test.go:39", // location where `checkAgeHandler` is declared.
			},
			{
				"url with domain name",
				"https://localhost/check/2625",
				"/check/:age/",
				MethodAll,
				"ong/mux/mux_test.go:39", // location where `checkAgeHandler` is declared.
			},
		}

		for _, tt := range tests {
			tt := tt

			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				rt := mux.Resolve(tt.path)
				attest.Equal(t, rt.method, tt.method)
				attest.Equal(t, rt.pattern, tt.pattern)
				attest.Subsequence(t, rt.String(), tt.stackPath)
			})
		}
	})

	t.Run("AddRoute", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		uri := "/api"
		httpsPort := tst.GetPort()
		domain := "localhost"
		mux := New(
			middleware.WithOpts(domain, httpsPort, tst.SecretKey(), middleware.DirectIpStrategy, l),
			nil,
			NewRoute(
				uri,
				MethodGet,
				someMuxHandler(msg),
			),
		)

		// {
		// 	const acmeChallengeURI = "/.well-known/acme-challenge/:token"
		// 	acmeHandler := acme.Handler(mux)
		// 	mux.AddRoute(
		// 		NewRoute(
		// 			acmeChallengeURI,
		// 			MethodAll,
		// 			acmeHandler,
		// 		),
		// 	)

		// 	fmt.Println("mux: ", mux.router)
		// }

		ts, err := tst.TlsServer(mux, domain, httpsPort)
		attest.Ok(t, err)
		defer ts.Close()

		res, err := client.Get(ts.URL + uri)
		attest.Ok(t, err)

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)
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

	l := log.New(context.Background(), &bytes.Buffer{}, 500)

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		mux := New(
			middleware.WithOpts("localhost", 443, tst.SecretKey(), middleware.DirectIpStrategy, l),
			nil,
			getManyRoutes()...,
		)
		r = mux
	}
	// always store the result to a package level variable
	// so the compiler cannot eliminate the Benchmark itself.
	result = r
}
