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

	"github.com/komuw/ong/internal/tst"

	"go.akshayshah.org/attest"
)

func someHttpsRedirectorHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			p := make([]byte, 16)
			_, err := r.Body.Read(p)
			if err == nil || err == io.EOF {
				_, _ = w.Write(p)
				return
			}
		}

		fmt.Fprint(w, msg)
	}
}

const locationHeader = "Location"

func TestHttpsRedirector(t *testing.T) {
	t.Parallel()

	tr := &http.Transport{
		// since we are using self-signed certificates, we need to skip verification.
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	t.Run("get is redirected to https", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		port := uint16(443)
		wrappedHandler := httpsRedirector(someHttpsRedirectorHandler(msg), port, "localhost")

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.Host = "localhost"
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusPermanentRedirect)
		attest.NotZero(t, res.Header.Get(locationHeader))
		attest.Equal(t, res.Header.Get(locationHeader), "https://localhost"+"/someUri")
	})

	t.Run("post is redirected to https", func(t *testing.T) {
		t.Parallel()

		msg := "hello you"
		port := uint16(443)
		wrappedHandler := httpsRedirector(someHttpsRedirectorHandler(msg), port, "localhost")
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/someUri", nil)
		req.Host = "localhost"
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusPermanentRedirect)
		attest.NotZero(t, res.Header.Get(locationHeader))
		attest.Equal(t, res.Header.Get(locationHeader), "https://localhost"+"/someUri")
	})

	t.Run("uri combinations", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		port := uint16(443)
		wrappedHandler := httpsRedirector(someHttpsRedirectorHandler(msg), port, "localhost")

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
			req.Host = "localhost"
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			attest.Equal(t, res.StatusCode, http.StatusPermanentRedirect)
			attest.NotZero(t, res.Header.Get(locationHeader))
			attest.Equal(t, res.Header.Get(locationHeader), "https://localhost"+uri)
		}
	})

	t.Run("get with tls succeds", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		httpsPort := tst.GetPort()
		domain := "localhost"
		wrappedHandler := httpsRedirector(someHttpsRedirectorHandler(msg), httpsPort, domain)

		ts, err := tst.TlsServer(wrappedHandler, domain, httpsPort)
		attest.Ok(t, err)
		defer ts.Close()

		res, err := client.Get(ts.URL)
		attest.Ok(t, err)
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Zero(t, res.Header.Get(locationHeader))
		attest.Equal(t, string(rb), msg)
	})

	t.Run("post with tls succeds", func(t *testing.T) {
		t.Parallel()

		// this test also asserts that a http POST is not converted to a http GET
		// as might happen if `httpsRedirector` was using `http.StatusMovedPermanently`

		msg := "hello world"
		httpsPort := tst.GetPort()
		domain := "localhost"
		wrappedHandler := httpsRedirector(someHttpsRedirectorHandler(msg), httpsPort, domain)

		ts, err := tst.TlsServer(wrappedHandler, domain, httpsPort)
		attest.Ok(t, err)
		defer ts.Close()

		postMsg := "my name is John"
		body := strings.NewReader(postMsg)
		res, err := client.Post(ts.URL, "application/json", body)
		attest.Ok(t, err)
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Zero(t, res.Header.Get(locationHeader))
		attest.False(t, strings.Contains(string(rb), msg))
		attest.Subsequence(t, string(rb), postMsg)
	})

	t.Run("port combinations", func(t *testing.T) {
		t.Parallel()

		uri := "/someUri"
		msg := "hello world"
		domain := "localhost"
		for _, p := range []uint16{
			uint16(443),
			uint16(80),
			uint16(88),
			uint16(65535),
		} {
			wrappedHandler := httpsRedirector(someHttpsRedirectorHandler(msg), p, domain)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, uri, nil)
			req.Host = domain
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			attest.Equal(t, res.StatusCode, http.StatusPermanentRedirect)
			attest.NotZero(t, res.Header.Get(locationHeader))

			expectedLocation := "https://localhost" + uri
			if p == uint16(88) || p == uint16(65535) {
				expectedLocation = "https://localhost" + ":" + fmt.Sprint(p) + uri
			}
			attest.Equal(t, res.Header.Get(locationHeader), expectedLocation)
		}
	})

	t.Run("IP is redirected to domain", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"

		httpsPort := tst.GetPort()
		domain := "localhost"
		wrappedHandler := httpsRedirector(someHttpsRedirectorHandler(msg), httpsPort, domain)

		ts, errTls := tst.TlsServer(wrappedHandler, domain, httpsPort)
		attest.Ok(t, errTls)
		defer ts.Close()

		{
			// use IP address(ie, `127.0.0.1"`)
			url := ts.URL
			url = strings.ReplaceAll(url, "localhost", "127.0.0.1")
			res, err := client.Get(url)
			attest.Ok(t, err)
			defer res.Body.Close()

			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Zero(t, res.Header.Get(locationHeader))
			attest.Equal(t, string(rb), msg)
		}

		{
			// use domain name(ie, `localhost`)
			url := ts.URL
			url = strings.ReplaceAll(url, "127.0.0.1", "localhost")
			res, err := client.Get(url)
			attest.Ok(t, err)
			defer res.Body.Close()

			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Zero(t, res.Header.Get(locationHeader))
			attest.Equal(t, string(rb), msg)
		}
	})

	t.Run("dns rebinding protection", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		port := uint16(443)
		domain := "localhost"
		wrappedHandler := httpsRedirector(someHttpsRedirectorHandler(msg), port, domain)
		ts := httptest.NewTLSServer(
			wrappedHandler,
		)
		t.Cleanup(func() {
			ts.Close()
		})

		tests := []struct {
			name         string
			host         string
			expectedCode int
			expectedMsg  string
		}{
			{
				name:         "good host",
				host:         domain,
				expectedCode: http.StatusOK,
				expectedMsg:  msg,
			},
			{
				name:         "good subdomain",
				host:         "subdomain." + domain,
				expectedCode: http.StatusOK,
				expectedMsg:  msg,
			},
			{
				name:         "bad host",
				host:         "example.com",
				expectedCode: http.StatusNotFound,
				expectedMsg:  "has an unexpected value",
			},
		}
		for _, tt := range tests {
			tt := tt
			_ = tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
				attest.Ok(t, err)
				req.Header.Set("Host", tt.host)
				req.Host = tt.host

				res, err := client.Do(req)
				attest.Ok(t, err)
				defer res.Body.Close()

				rb, err := io.ReadAll(res.Body)
				attest.Ok(t, err)

				attest.Equal(t, res.StatusCode, tt.expectedCode)
				attest.Zero(t, res.Header.Get(locationHeader))
				attest.Subsequence(t, string(rb), tt.expectedMsg)
			})
		}
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		port := uint16(443)
		// for this concurrency test, we have to re-use the same wrappedHandler
		// so that state is shared and thus we can see if there is any state which is not handled correctly.
		wrappedHandler := httpsRedirector(someHttpsRedirectorHandler(msg), port, "localhost")

		runhandler := func() {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/someUri", nil)
			req.Host = "localhost"
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
