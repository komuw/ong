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
	"github.com/komuw/ong/cry"
	"github.com/komuw/ong/id"
)

func getSecretKey() string {
	key := "hard-password"
	return key
}

func TestGetToken(t *testing.T) {
	t.Parallel()

	t.Run("empty request", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		tok := getToken(req)
		attest.Zero(t, tok)
	})

	t.Run("small token size", func(t *testing.T) {
		t.Parallel()
		want := id.Random(csrfBytesTokenLength / 2)
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.AddCookie(&http.Cookie{
			Name:     csrfCookieName,
			Value:    want,
			Path:     "/",
			HttpOnly: false, // If true, makes cookie inaccessible to JS. Should be false for csrf cookies.
			Secure:   true,  // https only.
			SameSite: http.SameSiteStrictMode,
		})
		tok := getToken(req)
		attest.Zero(t, tok)
	})

	t.Run("from cookie", func(t *testing.T) {
		t.Parallel()

		want := id.Random(csrfBytesTokenLength)
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.AddCookie(&http.Cookie{
			Name:     csrfCookieName,
			Value:    want,
			Path:     "/",
			HttpOnly: false, // If true, makes cookie inaccessible to JS. Should be false for csrf cookies.
			Secure:   true,  // https only.
			SameSite: http.SameSiteStrictMode,
		})
		got := getToken(req)
		attest.Equal(t, got, want)
	})

	t.Run("from header", func(t *testing.T) {
		t.Parallel()

		want := id.Random(csrfBytesTokenLength)
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.Header.Set(CsrfHeader, want)
		got := getToken(req)
		attest.Equal(t, got, want)
	})

	t.Run("from form", func(t *testing.T) {
		t.Parallel()

		want := id.Random(2 * csrfBytesTokenLength)
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		err := req.ParseForm()
		attest.Ok(t, err)
		req.Form.Add(CsrfTokenFormName, want)
		got := getToken(req)
		attest.Equal(t, got, want)
	})

	t.Run("cookie takes precedence", func(t *testing.T) {
		t.Parallel()

		cookieToken := id.Random(2 * csrfBytesTokenLength)
		headerToken := id.Random(2 * csrfBytesTokenLength)
		formToken := id.Random(2 * csrfBytesTokenLength)
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.AddCookie(&http.Cookie{
			Name:     csrfCookieName,
			Value:    cookieToken,
			Path:     "/",
			HttpOnly: false, // If true, makes cookie inaccessible to JS. Should be false for csrf cookies.
			Secure:   true,  // https only.
			SameSite: http.SameSiteStrictMode,
		})
		req.Header.Set(CsrfHeader, headerToken)
		err := req.ParseForm()
		attest.Ok(t, err)
		req.Form.Add(CsrfTokenFormName, formToken)

		got := getToken(req)
		attest.Equal(t, got, cookieToken)
	})
}

const tokenHeader = "CUSTOM-CSRF-TOKEN-TEST-HEADER"

func someCsrfHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(tokenHeader, GetCsrfToken(r.Context()))
		fmt.Fprint(w, msg)
	}
}

func TestCsrf(t *testing.T) {
	t.Parallel()

	t.Run("middleware succeds", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "example.com"
		wrappedHandler := csrf(someCsrfHandler(msg), getSecretKey(), domain)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)
	})

	t.Run("fetch token from GET requests", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "example.com"
		wrappedHandler := csrf(someCsrfHandler(msg), getSecretKey(), domain)

		reqCsrfTok := id.Random(csrfBytesTokenLength)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.AddCookie(&http.Cookie{
			Name:     csrfCookieName,
			Value:    reqCsrfTok,
			Path:     "/",
			HttpOnly: false, // If true, makes cookie inaccessible to JS. Should be false for csrf cookies.
			Secure:   true,  // https only.
			SameSite: http.SameSiteStrictMode,
		})
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)
		attest.NotZero(t, res.Header.Get(tokenHeader))
	})

	t.Run("fetch token from HEAD requests", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "example.com"
		wrappedHandler := csrf(someCsrfHandler(msg), getSecretKey(), domain)

		reqCsrfTok := id.Random(csrfBytesTokenLength)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodHead, "/someUri", nil)
		req.Header.Set(CsrfHeader, reqCsrfTok)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)
		attest.NotZero(t, res.Header.Get(tokenHeader))
	})

	t.Run("can generate csrf tokens", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "example.com"
		wrappedHandler := csrf(someCsrfHandler(msg), getSecretKey(), domain)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)
		attest.NotZero(t, res.Header.Get(tokenHeader))
	})

	t.Run("token is set in all required places", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "example.com"
		wrappedHandler := csrf(someCsrfHandler(msg), getSecretKey(), domain)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)

		// assert that:
		// (a) csrf cookie is set.
		// (b) cookie header is set.
		// (c) vary header is updated.
		// (d) r.context is updated.

		// (a)
		attest.Equal(t, len(res.Cookies()), 1)
		attest.Equal(t, res.Cookies()[0].Name, csrfCookieName)
		attest.Equal(t, res.Cookies()[0].Value, res.Header.Get(tokenHeader))

		// (b)
		attest.Equal(t, res.Header.Get(CsrfHeader), res.Header.Get(tokenHeader))

		// (c)
		attest.Equal(t, res.Header.Get(varyHeader), clientCookieHeader)

		// (d)
		attest.NotZero(t, res.Header.Get(tokenHeader))
	})

	t.Run("POST requests must have valid token", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "example.com"
		wrappedHandler := csrf(someCsrfHandler(msg), getSecretKey(), domain)

		reqCsrfTok := id.Random(csrfBytesTokenLength)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/someUri", nil)
		req.AddCookie(&http.Cookie{
			Name:     csrfCookieName,
			Value:    reqCsrfTok,
			Path:     "/",
			HttpOnly: false, // If true, makes cookie inaccessible to JS. Should be false for csrf cookies.
			Secure:   true,  // https only.
			SameSite: http.SameSiteStrictMode,
		})
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		// it is redirected.
		attest.Equal(t, res.StatusCode, http.StatusSeeOther)
		attest.Zero(t, res.Header.Get(tokenHeader))
		attest.Equal(t, len(res.Cookies()), 0)
	})

	t.Run("POST requests with valid token one tab", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "example.com"
		wrappedHandler := csrf(someCsrfHandler(msg), getSecretKey(), domain)

		key := getSecretKey()
		enc2 := cry.New(key)
		reqCsrfTok := enc2.EncryptEncode("msgToEncrypt")

		{
			// make GET request
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			req.Header.Set(CsrfHeader, reqCsrfTok)
			wrappedHandler.ServeHTTP(rec, req)
			res := rec.Result()
			defer res.Body.Close()
			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), msg)
		}

		{
			// make POST request using same csrf token
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/someUri", nil)
			req.AddCookie(&http.Cookie{
				Name:     csrfCookieName,
				Value:    reqCsrfTok,
				Path:     "/",
				HttpOnly: false, // If true, makes cookie inaccessible to JS. Should be false for csrf cookies.
				Secure:   true,  // https only.
				SameSite: http.SameSiteStrictMode,
			})
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)
			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), msg)

			// assert that:
			// (a) csrf cookie is set.
			// (b) cookie header is set.
			// (c) vary header is updated.
			// (d) r.context is updated.

			// (a)
			attest.Equal(t, len(res.Cookies()), 1)
			attest.Equal(t, res.Cookies()[0].Name, csrfCookieName)
			attest.Equal(t, res.Cookies()[0].Value, res.Header.Get(tokenHeader))

			// (b)
			attest.Equal(t, res.Header.Get(CsrfHeader), res.Header.Get(tokenHeader))

			// (c)
			attest.Equal(t, res.Header.Get(varyHeader), clientCookieHeader)

			// (d)
			attest.NotZero(t, res.Header.Get(tokenHeader))
		}
	})

	t.Run("POST requests with no cookies dont need csrf", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "example.com"
		wrappedHandler := csrf(someCsrfHandler(msg), getSecretKey(), domain)

		rec := httptest.NewRecorder()
		postMsg := "my name is John"
		body := strings.NewReader(postMsg)
		req := httptest.NewRequest(http.MethodPost, "/someUri", body)
		req.Header.Add(ctHeader, "application/json")
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)
		attest.NotZero(t, res.Header.Get(tokenHeader))
		attest.Equal(t, len(res.Cookies()), 1)
	})

	// concurrency safe
	t.Run("POST requests with valid token from mutiple tabs", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "example.com"
		// for this concurrency test, we have to re-use the same wrappedHandler
		// so that state is shared and thus we can see if there is any state which is not handled correctly.
		wrappedHandler := csrf(someCsrfHandler(msg), getSecretKey(), domain)

		key := getSecretKey()
		enc2 := cry.New(key)
		reqCsrfTok := enc2.EncryptEncode("msgToEncrypt")

		{
			// make GET request
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			req.Header.Set(CsrfHeader, reqCsrfTok)
			wrappedHandler.ServeHTTP(rec, req)
			res := rec.Result()
			defer res.Body.Close()
			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), msg)
		}

		{
			runTestPerTab := func() {
				// make POST request using same csrf token
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/someUri", nil)
				req.AddCookie(&http.Cookie{
					Name:     csrfCookieName,
					Value:    reqCsrfTok,
					Path:     "/",
					HttpOnly: false, // If true, makes cookie inaccessible to JS. Should be false for csrf cookies.
					Secure:   true,  // https only.
					SameSite: http.SameSiteStrictMode,
				})
				wrappedHandler.ServeHTTP(rec, req)

				res := rec.Result()
				defer res.Body.Close()

				rb, err := io.ReadAll(res.Body)
				attest.Ok(t, err)
				attest.Equal(t, res.StatusCode, http.StatusOK)
				attest.Equal(t, string(rb), msg)

				// assert that:
				// (a) csrf cookie is set.
				// (b) cookie header is set.
				// (c) vary header is updated.
				// (d) r.context is updated.

				// (a)
				attest.Equal(t, len(res.Cookies()), 1)
				attest.Equal(t, res.Cookies()[0].Name, csrfCookieName)
				attest.Equal(t, res.Cookies()[0].Value, res.Header.Get(tokenHeader))

				// (b)
				attest.Equal(t, res.Header.Get(CsrfHeader), res.Header.Get(tokenHeader))

				// (c)
				attest.Equal(t, res.Header.Get(varyHeader), clientCookieHeader)

				// (d)
				attest.NotZero(t, res.Header.Get(tokenHeader))
			}

			wg := &sync.WaitGroup{}
			for tabNumber := 0; tabNumber <= 5; tabNumber++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					runTestPerTab()
				}()
			}
			wg.Wait()
		}
	})
}
