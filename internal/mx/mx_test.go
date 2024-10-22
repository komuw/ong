package mx

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/komuw/ong/config"
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
	_, err := NewRoute(
		"/api",
		MethodGet,
		someMuxHandler("msg"),
	)
	attest.Ok(t, err)

	basicAuth, err := middleware.BasicAuth(someMuxHandler("msg"), "some-user", "some-very-very-h1rd-passwd")
	attest.Ok(t, err)

	// succeds
	_, errA := NewRoute(
		"/api",
		MethodGet,
		basicAuth,
	)
	attest.Ok(t, errA)

	// fails
	_, errB := NewRoute(
		"/api",
		MethodGet,
		middleware.Get(
			someMuxHandler("msg"),
			config.WithOpts("localhost", 443, tst.SecretKey(), config.DirectIpStrategy, l),
		),
	)
	attest.Error(t, errB)
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
		rt, err := NewRoute(
			"/api",
			MethodGet,
			someMuxHandler(msg),
		)
		attest.Ok(t, err)
		mux, err := New(
			config.WithOpts("localhost", 443, tst.SecretKey(), config.DirectIpStrategy, l),
			nil,
			rt,
		)
		attest.Ok(t, err)

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
		rt, err := NewRoute(
			uri,
			MethodGet,
			someMuxHandler(msg),
		)
		attest.Ok(t, err)
		mux, err := New(
			config.WithOpts(domain, httpsPort, tst.SecretKey(), config.DirectIpStrategy, l),
			nil,
			rt,
		)
		attest.Ok(t, err)

		ts, errTlS := tst.TlsServer(mux, domain, httpsPort)
		attest.Ok(t, errTlS)
		defer ts.Close()

		csrfToken := ""
		{
			// non-safe http methods(like POST) require a server-known csrf token;
			// otherwise it fails with http 403
			// so here we make a http GET so that we can have a csrf token.
			res, errG := client.Get(ts.URL + uri)
			attest.Ok(t, errG)
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
		rt, err := NewRoute(
			uri,
			MethodGet,
			someMuxHandler(msg),
		)
		attest.Ok(t, err)
		mux, err := New(
			config.WithOpts(domain, httpsPort, tst.SecretKey(), config.DirectIpStrategy, l),
			nil,
			rt,
		)
		attest.Ok(t, err)

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
		uri2 := "/api/:someId" // This conflicts with uri1
		method := MethodGet

		rt1, err := NewRoute(
			uri1,
			method,
			someMuxHandler(msg),
		)
		attest.Ok(t, err)

		rt2, err := NewRoute(
			uri2,
			method,
			thisIsAnotherMuxHandler(),
		)
		attest.Ok(t, err)

		_, errC := New(
			config.WithOpts("localhost", 443, tst.SecretKey(), config.DirectIpStrategy, l),
			nil,
			rt1,
			rt2,
		)
		attest.Error(t, errC)
		rStr := errC.Error()
		attest.Subsequence(t, rStr, uri2)
		attest.Subsequence(t, rStr, method)
		attest.Subsequence(t, rStr, "ong/internal/mx/mx_test.go:28") // location where `someMuxHandler` is declared.
		attest.Subsequence(t, rStr, "ong/internal/mx/mx_test.go:34") // location where `thisIsAnotherMuxHandler` is declared.
	})

	t.Run("resolve url", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		expectedHandler := someMuxHandler(msg)
		rt1, err := NewRoute(
			"/api",
			MethodGet,
			expectedHandler,
		)
		attest.Ok(t, err)
		rt2, err := NewRoute(
			"check/:age/",
			MethodAll,
			checkAgeHandler(),
		)
		attest.Ok(t, err)
		mux, err := New(
			config.WithOpts("localhost", 443, tst.SecretKey(), config.DirectIpStrategy, l),
			nil,
			rt1,
			rt2,
		)
		attest.Ok(t, err)

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
				"ong/internal/mx/mx_test.go:28", // location where `someMuxHandler` is declared.
			},
			{
				"success with prefix slash",
				"/api",
				"/api/",
				MethodGet,
				"ong/internal/mx/mx_test.go:28", // location where `someMuxHandler` is declared.
			},
			{
				"success with suffix slash",
				"api/",
				"/api/",
				MethodGet,
				"ong/internal/mx/mx_test.go:28", // location where `someMuxHandler` is declared.
			},
			{
				"success with all slashes",
				"/api/",
				"/api/",
				MethodGet,
				"ong/internal/mx/mx_test.go:28", // location where `someMuxHandler` is declared.
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
				"ong/internal/mx/mx_test.go:40", // location where `checkAgeHandler` is declared.
			},
			{
				"url with domain name",
				"https://localhost/check/2625",
				"/check/:age/",
				MethodAll,
				"ong/internal/mx/mx_test.go:40", // location where `checkAgeHandler` is declared.
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
		rt, err := NewRoute(
			uri,
			MethodGet,
			someMuxHandler(msg),
		)
		attest.Ok(t, err)
		mux, err := New(
			config.WithOpts(domain, httpsPort, tst.SecretKey(), config.DirectIpStrategy, l),
			nil,
			rt,
		)
		attest.Ok(t, err)

		{
			someOtherMuxHandler := func(msg string) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprint(w, msg)
				}
			}

			msg2 := "someOtherMuxHandler"
			rt2, errN := NewRoute(
				"/someOtherMuxHandler",
				MethodAll,
				someOtherMuxHandler(msg2),
			)
			attest.Ok(t, errN)
			attest.Ok(t, mux.AddRoute(rt2))
		}

		{ // detects conflicts
			rt3, errNr := NewRoute(
				uri,
				MethodGet,
				someMuxHandler(msg),
			)
			attest.Ok(t, errNr)
			errA := mux.AddRoute(rt3)
			attest.Error(t, errA)
		}

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

		{
			res2, errCg := client.Get(ts.URL + "/someOtherMuxHandler")
			attest.Ok(t, errCg)

			rb2, errRa := io.ReadAll(res2.Body)
			attest.Ok(t, errRa)
			defer res2.Body.Close()

			attest.Equal(t, res2.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb2), "someOtherMuxHandler")
		}
	})

	t.Run("merge", func(t *testing.T) {
		t.Parallel()

		httpsPort := tst.GetPort()
		domain := "localhost"

		t.Run("success", func(t *testing.T) {
			t.Parallel()
			rt1, err := NewRoute("/abc", MethodGet, someMuxHandler("hello"))
			attest.Ok(t, err)

			mux1, err := New(config.WithOpts(domain, httpsPort, tst.SecretKey(), config.DirectIpStrategy, l), nil, rt1)
			attest.Ok(t, err)

			rt2, err := NewRoute("/ijk", MethodGet, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
			attest.Ok(t, err)
			rt3, err := NewRoute("/xyz", MethodGet, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
			attest.Ok(t, err)

			mux2, err := New(config.WithOpts(domain, httpsPort, tst.SecretKey(), config.DirectIpStrategy, l), nil, rt2, rt3)
			attest.Ok(t, err)

			m, err := mux1.Merge([]Muxer{mux2})
			attest.Ok(t, err)

			attest.Equal(t, m.opt, mux1.opt)
			attest.Equal(t, fmt.Sprintf("%p", m.router.notFoundHandler), fmt.Sprintf("%p", mux1.router.notFoundHandler))
			attest.Equal(t, len(m.router.routes), 3)
		})

		t.Run("conflict", func(t *testing.T) {
			rt1, err := NewRoute("/abc", MethodGet, someMuxHandler("hello"))
			attest.Ok(t, err)

			mux1, err := New(config.WithOpts(domain, httpsPort, tst.SecretKey(), config.DirectIpStrategy, l), nil, rt1)
			attest.Ok(t, err)

			rt2, err := NewRoute("/ijk", MethodGet, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
			attest.Ok(t, err)
			rt3, err := NewRoute("/abc", MethodGet, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
			attest.Ok(t, err)

			mux2, err := New(config.WithOpts(domain, httpsPort, tst.SecretKey(), config.DirectIpStrategy, l), nil, rt2, rt3)
			attest.Ok(t, err)

			_, errM := mux1.Merge([]Muxer{mux2})
			attest.Error(t, errM)
			rStr := errM.Error()
			attest.Subsequence(t, rStr, "would conflict")
			attest.Subsequence(t, rStr, "ong/internal/mx/mx_test.go:28")  // location where `someMuxHandler` is declared.
			attest.Subsequence(t, rStr, "ong/internal/mx/mx_test.go:449") // location where the other handler is declared.
		})
	})
}

func TestMuxFlexiblePattern(t *testing.T) {
	t.Parallel()

	tr := &http.Transport{
		// since we are using self-signed certificates, we need to skip verification.
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	httpsPort := tst.GetPort()
	domain := "localhost"

	l := log.New(context.Background(), &bytes.Buffer{}, 500)

	t.Run("flexible pattern accepts all uris", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		rt, err := NewRoute(
			"/*",
			MethodGet,
			someMuxHandler(msg),
		)
		attest.Ok(t, err)
		mux, err := New(
			config.WithOpts(domain, httpsPort, tst.SecretKey(), config.DirectIpStrategy, l),
			nil,
			rt,
		)
		attest.Ok(t, err)

		ts, err := tst.TlsServer(mux, domain, httpsPort)
		attest.Ok(t, err)
		defer ts.Close()

		{
			res, errA := client.Get(ts.URL + "/UnknownUri")
			attest.Ok(t, errA)

			rb, errB := io.ReadAll(res.Body)
			attest.Ok(t, errB)

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), msg)
		}

		{
			res, errC := client.Get(ts.URL + "/")
			attest.Ok(t, errC)

			rb, errD := io.ReadAll(res.Body)
			attest.Ok(t, errD)

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), msg)
		}

		{
			res, errE := client.Get(ts.URL + "/hey/a/b/cool")
			attest.Ok(t, errE)

			rb, errF := io.ReadAll(res.Body)
			attest.Ok(t, errF)

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), msg)
		}
	})

	t.Run("conflict", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"

		rt1, err := NewRoute(
			"/*",
			MethodGet,
			someMuxHandler(msg),
		)
		attest.Ok(t, err)

		rt2, err := NewRoute(
			"/hi",
			MethodGet,
			thisIsAnotherMuxHandler(),
		)
		attest.Ok(t, err)

		_, err = New(
			config.WithOpts(domain, httpsPort, tst.SecretKey(), config.DirectIpStrategy, l),
			nil,
			rt1,
			rt2,
		)
		attest.Error(t, err)
		attest.Subsequence(t, err.Error(), "would conflict")
	})
}

func getManyRoutes(b *testing.B) []Route {
	b.Helper()

	routes := []Route{}

	for i := 0; i <= 200; i++ {
		uri := fmt.Sprintf("uri-%d", i)
		rt, err := NewRoute(
			uri,
			MethodAll,
			someMuxHandler(uri),
		)
		attest.Ok(b, err)
		routes = append(
			routes,
			rt,
		)
	}

	return routes
}

var result Muxer //nolint:gochecknoglobals

func BenchmarkMuxNew(b *testing.B) {
	var r Muxer

	l := log.New(context.Background(), &bytes.Buffer{}, 500)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		mux, err := New(
			config.WithOpts("localhost", 443, tst.SecretKey(), config.DirectIpStrategy, l),
			nil,
			getManyRoutes(b)...,
		)
		attest.Ok(b, err)
		r = mux
	}
	// always store the result to a package level variable
	// so the compiler cannot eliminate the Benchmark itself.
	result = r
}
