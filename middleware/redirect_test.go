package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

	t.Run("get is redirected", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		port := uint16(443)
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
		port := uint16(443)
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
		port := uint16(443)
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
		port := uint16(443)
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
		port := uint16(443)
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

	t.Run("port combinations", func(t *testing.T) {
		t.Parallel()

		uri := "/someUri"
		msg := "hello world"
		for _, p := range []uint16{
			uint16(443),
			uint16(80),
			uint16(88),
			uint16(65535),
		} {
			wrappedHandler := HttpsRedirector(someHttpsRedirectorHandler(msg), p)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, uri, nil)
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			attest.Equal(t, res.StatusCode, http.StatusPermanentRedirect)
			attest.NotZero(t, res.Header.Get(locationHeader))

			expectedLocation := "https://example.com" + uri
			if p == uint16(88) || p == uint16(65535) {
				expectedLocation = "https://example.com" + ":" + fmt.Sprint(p) + uri
			}
			attest.Equal(t, res.Header.Get(locationHeader), expectedLocation)
		}
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		port := uint16(443)
		// for this concurrency test, we have to re-use the same wrappedHandler
		// so that state is shared and thus we can see if there is any state which is not handled correctly.
		wrappedHandler := HttpsRedirector(someHttpsRedirectorHandler(msg), port)

		runhandler := func() {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/someUri", nil)
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			attest.Equal(t, res.StatusCode, http.StatusPermanentRedirect)
			attest.NotZero(t, res.Header.Get(locationHeader))
		}

		wg := &sync.WaitGroup{}
		for rN := 0; rN <= 14; rN++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				runhandler()
			}()
		}
		wg.Wait()
	})
}
