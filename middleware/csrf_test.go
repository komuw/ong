package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/akshayjshah/attest"
	"github.com/komuw/goweb/id"
)

func TestStore(t *testing.T) {
	t.Parallel()

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		store := newStore()

		tokens := []string{
			"a", "aa", "aaa", "aaron", "ab", "abandoned", "abc", "aberdeen", "abilities", "ability", "able", "aboriginal", "abortion",
			"about", "above", "abraham", "abroad", "abs", "absence", "absent", "absolute", "absolutely", "absorption", "abstract",
			"abstracts", "abu", "abuse", "ac", "academic", "academics", "academy", "acc", "accent", "accept", "acceptable", "acceptance",
			"accepted", "accepting", "accepts", "access", "accessed", "accessibility", "accessible", "accessing", "accessories",
			"accessory", "accident", "accidents", "accommodate", "accommodation", "accommodations", "accompanied", "accompanying",
			"accomplish", "accomplished", "accordance", "according", "accordingly", "account", "accountability", "accounting", "accounts",
			"accreditation", "accredited", "accuracy", "accurate", "accurately", "accused", "acdbentity", "ace",
		}

		for _, tok := range tokens {
			go func(t string) {
				store.set(t)
			}(tok)
		}

		for _, tok := range tokens {
			go func(t string) {
				store.exists(t)
			}(tok)
		}

		for _, tok := range tokens {
			go func(t string) {
				store.reset()
			}(tok)
		}

		wg := &sync.WaitGroup{}
		for _, tok := range tokens {
			wg.Add(1)
			go func(t string) {
				store.set(t)
				wg.Done()
			}(tok)
		}
		wg.Wait()
	})

	t.Run("set", func(t *testing.T) {
		t.Parallel()

		store := newStore()

		tokens := []string{
			"a", "aa", "aaa", "aaron", "ab", "abandoned", "abc", "aberdeen", "abilities", "ability", "able", "aboriginal", "abortion",
			"about", "above", "abraham", "abroad", "abs", "absence", "absent", "absolute", "absolutely", "absorption", "abstract",
			"abstracts", "abu", "abuse", "ac", "academic", "academics", "academy", "acc", "accent", "accept", "acceptable", "acceptance",
			"accepted", "accepting", "accepts", "access", "accessed", "accessibility", "accessible", "accessing", "accessories",
			"accessory", "accident", "accidents", "accommodate", "accommodation", "accommodations", "accompanied", "accompanying",
			"accomplish", "accomplished", "accordance", "according", "accordingly", "account", "accountability", "accounting", "accounts",
			"accreditation", "accredited", "accuracy", "accurate", "accurately", "accused", "acdbentity", "ace",
		}
		wg := &sync.WaitGroup{}
		for _, tok := range tokens {
			wg.Add(1)
			go func(t string) {
				store.set(t)
				wg.Done()
			}(tok)
		}
		wg.Wait()

		attest.Equal(t, store._len(), len(tokens))
	})

	t.Run("reset", func(t *testing.T) {
		t.Parallel()

		store := newStore()

		tokens := []string{"aaron", "abandoned", "according", "accreditation", "accurately", "accused"}
		wg := &sync.WaitGroup{}
		for _, tok := range tokens {
			wg.Add(1)
			go func(t string) {
				store.set(t)
				wg.Done()
			}(tok)
		}
		wg.Wait()

		attest.Equal(t, store._len(), len(tokens))

		store.reset()
		attest.Equal(t, store._len(), 0)
	})
}

func TestGetToken(t *testing.T) {
	t.Parallel()

	t.Run("empty request", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
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
		req.Header.Set(csrfHeader, want)
		got := getToken(req)
		attest.Equal(t, got, want)
	})

	t.Run("from form", func(t *testing.T) {
		t.Parallel()

		want := id.Random(csrfBytesTokenLength)
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		err := req.ParseForm()
		attest.Ok(t, err)
		req.Form.Add(csrfCookieForm, want)
		got := getToken(req)
		attest.Equal(t, got, want)
	})

	t.Run("cookie takes precedence", func(t *testing.T) {
		t.Parallel()

		cookieToken := id.Random(csrfBytesTokenLength)
		headerToken := id.Random(csrfBytesTokenLength)
		formToken := id.Random(csrfBytesTokenLength)
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.AddCookie(&http.Cookie{
			Name:     csrfCookieName,
			Value:    cookieToken,
			Path:     "/",
			HttpOnly: false, // If true, makes cookie inaccessible to JS. Should be false for csrf cookies.
			Secure:   true,  // https only.
			SameSite: http.SameSiteStrictMode,
		})
		req.Header.Set(csrfHeader, headerToken)
		err := req.ParseForm()
		attest.Ok(t, err)
		req.Form.Add(csrfCookieForm, formToken)

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
		wrappedHandler := Csrf(someCsrfHandler(msg), domain)

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
		wrappedHandler := Csrf(someCsrfHandler(msg), domain)

		reqCsrfTok := id.Random(2 * csrfBytesTokenLength)
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
		attest.Equal(t, res.Header.Get(tokenHeader)[csrfStringTokenlength:], reqCsrfTok[csrfStringTokenlength:])
	})

	t.Run("fetch token from HEAD requests", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "example.com"
		wrappedHandler := Csrf(someCsrfHandler(msg), domain)

		reqCsrfTok := id.Random(2 * csrfBytesTokenLength)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodHead, "/someUri", nil)
		req.Header.Set(csrfHeader, reqCsrfTok)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)
		attest.Equal(t, res.Header.Get(tokenHeader)[csrfStringTokenlength:], reqCsrfTok[csrfStringTokenlength:])
	})

	t.Run("can generate csrf tokens", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "example.com"
		wrappedHandler := Csrf(someCsrfHandler(msg), domain)

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
		wrappedHandler := Csrf(someCsrfHandler(msg), domain)

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
		// (e) memory store is updated.

		// (a)
		attest.Equal(t, len(res.Cookies()), 1)
		attest.Equal(t, res.Cookies()[0].Name, csrfCookieName)
		attest.Equal(t, res.Cookies()[0].Value, res.Header.Get(tokenHeader))

		// (b)
		attest.Equal(t, res.Header.Get(csrfHeader), res.Header.Get(tokenHeader))

		// (c)
		attest.Equal(t, res.Header.Get(varyHeader), clientCookieHeader)

		// (d)
		attest.NotZero(t, res.Header.Get(tokenHeader))

		// (e)
		attest.True(t, csrfStore.exists(res.Header.Get(tokenHeader)[csrfStringTokenlength:]))
		attest.True(t, csrfStore._len() > 0)
	})

	t.Run("POST requests must have valid token", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "example.com"
		wrappedHandler := Csrf(someCsrfHandler(msg), domain)

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

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusForbidden)
		attest.Equal(t, string(rb), errCsrfTokenNotFound.Error()+"\n")
		attest.Zero(t, res.Header.Get(tokenHeader))
		attest.Equal(t, len(res.Cookies()), 0)
	})

	t.Run("POST requests with valid token", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "example.com"
		wrappedHandler := Csrf(someCsrfHandler(msg), domain)

		reqCsrfTok := id.Random(2 * csrfBytesTokenLength)

		{
			// make GET request
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			req.Header.Set(csrfHeader, reqCsrfTok)
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
			// (e) memory store is updated.

			// (a)
			attest.Equal(t, len(res.Cookies()), 1)
			attest.Equal(t, res.Cookies()[0].Name, csrfCookieName)
			attest.Equal(t, res.Cookies()[0].Value, res.Header.Get(tokenHeader))

			// (b)
			attest.Equal(t, res.Header.Get(csrfHeader), res.Header.Get(tokenHeader))

			// (c)
			attest.Equal(t, res.Header.Get(varyHeader), clientCookieHeader)

			// (d)
			attest.NotZero(t, res.Header.Get(tokenHeader))

			// (e)
			attest.True(t, csrfStore.exists(res.Header.Get(tokenHeader)[csrfStringTokenlength:]))
			attest.True(t, csrfStore._len() > 0)
		}
	})

	t.Run("POST requests with valid token from mutiple tabs", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "example.com"
		wrappedHandler := Csrf(someCsrfHandler(msg), domain)

		reqCsrfTok := id.Random(2 * csrfBytesTokenLength)

		{
			// make GET request
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			req.Header.Set(csrfHeader, reqCsrfTok)
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
				// (e) memory store is updated.

				// (a)
				attest.Equal(t, len(res.Cookies()), 1)
				attest.Equal(t, res.Cookies()[0].Name, csrfCookieName)
				attest.Equal(t, res.Cookies()[0].Value, res.Header.Get(tokenHeader))

				// (b)
				attest.Equal(t, res.Header.Get(csrfHeader), res.Header.Get(tokenHeader))

				// (c)
				attest.Equal(t, res.Header.Get(varyHeader), clientCookieHeader)

				// (d)
				attest.NotZero(t, res.Header.Get(tokenHeader))

				// (e)
				attest.True(t, csrfStore.exists(res.Header.Get(tokenHeader)[csrfStringTokenlength:]))
				attest.True(t, csrfStore._len() > 0)
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
