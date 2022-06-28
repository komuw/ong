package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/akshayjshah/attest"
)

func someHttpsRedirectorHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, msg)
	}
}

const locationHeader = "Location"

func TestHttpsRedirector(t *testing.T) {
	t.Parallel()

	t.Run("get is redirected", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := HttpsRedirector(someHttpsRedirectorHandler(msg))
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

		msg := "hello"
		wrappedHandler := HttpsRedirector(someHttpsRedirectorHandler(msg))
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

		msg := "hello"
		wrappedHandler := HttpsRedirector(someHttpsRedirectorHandler(msg))

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
			attest.Equal(t, res.Header.Get(locationHeader), uri)
		}
	})

	t.Run("get with tls succeds", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		wrappedHandler := HttpsRedirector(someHttpsRedirectorHandler(msg))
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
}
