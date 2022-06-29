package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/akshayjshah/attest"
)

func someHttpsRedirectorHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			p := make([]byte, 16)
			_, err := r.Body.Read(p)
			if err == nil || err == io.EOF {
				fmt.Fprint(w, string(p))
				return
			}
		}

		fmt.Fprint(w, msg)
	}
}

const locationHeader = "Location"

func TestHttpsRedirector(t *testing.T) {
	t.Parallel()

	// t.Run("different port combinations", func(t *testing.T) {
	// 	t.Parallel()

	// 	msg := "hello world"
	// 	port := "443" // "", "80", "78726", etc
	// 	wrappedHandler := HttpsRedirector(someHttpsRedirectorHandler(msg), port)
	// 	rec := httptest.NewRecorder()
	// 	req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
	// 	wrappedHandler.ServeHTTP(rec, req)

	// 	res := rec.Result()
	// 	defer res.Body.Close()

	// 	attest.Equal(t, res.StatusCode, http.StatusPermanentRedirect)
	// 	attest.NotZero(t, res.Header.Get(locationHeader))
	// })

	t.Run("get is redirected", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		port := "443"
		wrappedHandler := HttpsRedirector(someHttpsRedirectorHandler(msg), port)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusPermanentRedirect)
		attest.NotZero(t, res.Header.Get(locationHeader))
	})

	t.Run("post is redirected", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		port := "443"
		wrappedHandler := HttpsRedirector(someHttpsRedirectorHandler(msg), port)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusPermanentRedirect)
		attest.NotZero(t, res.Header.Get(locationHeader))
	})

	t.Run("uri combinations", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		port := "443"
		wrappedHandler := HttpsRedirector(someHttpsRedirectorHandler(msg), port)

		for _, uri := range []string{
			"/someUri",
			"/someUri/",
			//
			"/someUri/somethOther",
			"/someUri/somethOther/",
			//
			"/foo?bar",
			"/foo?bar/",
			//
			"/over/there?name=ferret",
			"/path/to/page?name=ferret&color=purple",
			//
			"/google/search?q=Wangari+Maathai&ei=JHSHS&ved=9Kjsh&uact=5&oq=Wangari+Maathai&gs_lcp=Mjandan-smmms&sclient=gws-wiz",
		} {
			uri := uri
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, uri, nil)
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			attest.Equal(t, res.StatusCode, http.StatusPermanentRedirect)
			attest.NotZero(t, res.Header.Get(locationHeader))
			attest.Equal(t, res.Header.Get(locationHeader), "https://example.com"+uri)
		}
	})

	t.Run("get with tls succeds", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		port := "443"
		wrappedHandler := HttpsRedirector(someHttpsRedirectorHandler(msg), port)
		ts := httptest.NewTLSServer(
			wrappedHandler,
		)
		defer ts.Close()

		client := ts.Client()
		res, err := client.Get(ts.URL)
		attest.Ok(t, err)

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Zero(t, res.Header.Get(locationHeader))
		attest.Equal(t, string(rb), msg)
	})

	t.Run("post with tls succeds", func(t *testing.T) {
		t.Parallel()

		// this test also asserts that a http POST is not converted to a http GET
		// as might happen if `HttpsRedirector` was using `http.StatusMovedPermanently`

		msg := "hello world"
		port := "443"
		wrappedHandler := HttpsRedirector(someHttpsRedirectorHandler(msg), port)
		ts := httptest.NewTLSServer(
			wrappedHandler,
		)
		defer ts.Close()

		client := ts.Client()
		postMsg := "my name is John"
		body := strings.NewReader(postMsg)
		res, err := client.Post(ts.URL, "application/json", body)
		attest.Ok(t, err)

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Zero(t, res.Header.Get(locationHeader))
		attest.True(t, !strings.Contains(string(rb), msg))
		attest.True(t, strings.Contains(string(rb), postMsg))
	})
}
