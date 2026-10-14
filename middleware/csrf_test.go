package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/komuw/ong/config"
	"github.com/komuw/ong/cry"
	"github.com/komuw/ong/id"
	"github.com/komuw/ong/internal/tst"

	"go.akshayshah.org/attest"
)

func csrfTestToken(clientID string, d time.Duration) string {
	once.Do(func() {
		enc = cry.New(tst.SecretKey())
	})
	expires := strconv.FormatInt(time.Now().UTC().Add(d).Unix(), 10)
	return enc.EncryptEncode(clientID + sep + expires)
}

func csrfTestCookie(tok string) *http.Cookie {
	return &http.Cookie{
		Name:     "__Host-" + csrfCookieName,
		Value:    tok,
		Path:     "/",
		HttpOnly: false, // If true, makes cookie inaccessible to JS. Should be false for csrf cookies.
		Secure:   true,  // https only.
		SameSite: http.SameSiteStrictMode,
	}
}

func TestGetTokens(t *testing.T) {
	t.Parallel()

	t.Run("empty request", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		attest.Zero(t, getCookieToken(req))
		attest.Zero(t, getRequestToken(req))
	})

	t.Run("from cookie", func(t *testing.T) {
		t.Parallel()

		want := id.Random(csrfBytesTokenLength)
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.AddCookie(csrfTestCookie(want))
		attest.Equal(t, getCookieToken(req), want)
		attest.Zero(t, getRequestToken(req))
	})

	t.Run("from header", func(t *testing.T) {
		t.Parallel()

		want := id.Random(csrfBytesTokenLength)
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.Header.Set(CsrfHeader, want)
		attest.Equal(t, getRequestToken(req), want)
		attest.Zero(t, getCookieToken(req))
	})

	t.Run("from form", func(t *testing.T) {
		t.Parallel()

		want := id.Random(2 * csrfBytesTokenLength)
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		err := req.ParseForm()
		attest.Ok(t, err)
		req.Form.Add(CsrfTokenFormName, want)
		attest.Equal(t, getRequestToken(req), want)
	})

	t.Run("header takes precedence over form", func(t *testing.T) {
		t.Parallel()

		headerToken := id.Random(2 * csrfBytesTokenLength)
		formToken := id.Random(2 * csrfBytesTokenLength)
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.Header.Set(CsrfHeader, headerToken)
		err := req.ParseForm()
		attest.Ok(t, err)
		req.Form.Add(CsrfTokenFormName, formToken)
		attest.Equal(t, getRequestToken(req), headerToken)
	})
}

func TestParseCsrfToken(t *testing.T) {
	t.Parallel()

	clientID := id.Random(csrfBytesTokenLength)
	tests := []struct {
		name    string
		tok     string
		wantErr error
	}{
		{name: "empty", tok: "", wantErr: errCsrfTokenNotFound},
		{name: "short", tok: id.Random(csrfBytesTokenLength / 2), wantErr: errCsrfTokenNotFound},
		{name: "junk", tok: id.Random(2 * csrfBytesTokenLength), wantErr: errCsrfTokenNotFound},
		{name: "wrong format", tok: csrfTestToken(clientID+sep+"extra", time.Hour), wantErr: errCsrfTokenWrongFormat},
		{name: "expired", tok: csrfTestToken(clientID, -time.Hour), wantErr: errCsrfTokenExpired},
		{name: "valid", tok: csrfTestToken(clientID, time.Hour), wantErr: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseCsrfToken(tt.tok)
			if tt.wantErr != nil {
				attest.ErrorIs(t, err, tt.wantErr)
				attest.Zero(t, got)
				return
			}
			attest.Ok(t, err)
			attest.Equal(t, got, clientID)
		})
	}
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
		wrappedHandler := csrf(someCsrfHandler(msg), tst.SecretKey(), config.DefaultCsrfCookieDuration)

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
		wrappedHandler := csrf(someCsrfHandler(msg), tst.SecretKey(), config.DefaultCsrfCookieDuration)

		reqCsrfTok := id.Random(csrfBytesTokenLength)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.AddCookie(&http.Cookie{
			Name:     "__Host-" + csrfCookieName,
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
		wrappedHandler := csrf(someCsrfHandler(msg), tst.SecretKey(), config.DefaultCsrfCookieDuration)

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
		wrappedHandler := csrf(someCsrfHandler(msg), tst.SecretKey(), config.DefaultCsrfCookieDuration)

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
		wrappedHandler := csrf(someCsrfHandler(msg), tst.SecretKey(), config.DefaultCsrfCookieDuration)

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
		attest.Equal(t, res.Cookies()[0].Name, "__Host-"+csrfCookieName)
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
		wrappedHandler := csrf(someCsrfHandler(msg), tst.SecretKey(), config.DefaultCsrfCookieDuration)

		reqCsrfTok := id.Random(csrfBytesTokenLength * 2)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/someUri", nil)
		req.AddCookie(&http.Cookie{
			Name:     "__Host-" + csrfCookieName,
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

	t.Run("POST requests with no cookies dont need csrf", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := csrf(someCsrfHandler(msg), tst.SecretKey(), config.DefaultCsrfCookieDuration)

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

	t.Run("POST requests with matching tokens", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := csrf(someCsrfHandler(msg), tst.SecretKey(), config.DefaultCsrfCookieDuration)

		clientID := id.Random(csrfBytesTokenLength)

		for _, inForm := range []bool{false, true} {
			cookieTok := csrfTestToken(clientID, config.DefaultCsrfCookieDuration)
			reqTok := csrfTestToken(clientID, config.DefaultCsrfCookieDuration)

			rec := httptest.NewRecorder()
			var req *http.Request
			if inForm {
				body := strings.NewReader(CsrfTokenFormName + "=" + reqTok)
				req = httptest.NewRequest(http.MethodPost, "/someUri", body)
				req.Header.Set(ctHeader, formUrlEncoded)
			} else {
				req = httptest.NewRequest(http.MethodPost, "/someUri", nil)
				req.Header.Set(CsrfHeader, reqTok)
			}
			req.AddCookie(csrfTestCookie(cookieTok))
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
			// (e) the new token has the same clientID.

			// (a)
			attest.Equal(t, len(res.Cookies()), 1)
			attest.Equal(t, res.Cookies()[0].Name, "__Host-"+csrfCookieName)
			attest.Equal(t, res.Cookies()[0].Value, res.Header.Get(tokenHeader))

			// (b)
			attest.Equal(t, res.Header.Get(CsrfHeader), res.Header.Get(tokenHeader))

			// (c)
			attest.Equal(t, res.Header.Get(varyHeader), clientCookieHeader)

			// (d)
			attest.NotZero(t, res.Header.Get(tokenHeader))

			// (e)
			newTok := res.Header.Get(CsrfHeader)
			attest.NotEqual(t, newTok, cookieTok)
			gotID, err := parseCsrfToken(newTok)
			attest.Ok(t, err)
			attest.Equal(t, gotID, clientID)
		}
	})

	t.Run("POST requests with bad tokens are rejected", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := csrf(someCsrfHandler(msg), tst.SecretKey(), config.DefaultCsrfCookieDuration)

		clientID := id.Random(csrfBytesTokenLength)
		validTok := csrfTestToken(clientID, config.DefaultCsrfCookieDuration)

		tests := []struct {
			name      string
			cookieTok string
			reqTok    string
			wantErr   error
		}{
			{
				name:      "request token with no cookie",
				cookieTok: "",
				reqTok:    validTok,
				wantErr:   errCsrfTokenNotFound,
			},
			{
				name:      "cookie with no request token",
				cookieTok: validTok,
				reqTok:    "",
				wantErr:   errCsrfTokenNotFound,
			},
			{
				name:      "cookie with junk request token",
				cookieTok: validTok,
				reqTok:    id.Random(2 * csrfBytesTokenLength),
				wantErr:   errCsrfTokenNotFound,
			},
			{
				name:      "tokens from different clients",
				cookieTok: validTok,
				reqTok:    csrfTestToken(id.Random(csrfBytesTokenLength), config.DefaultCsrfCookieDuration),
				wantErr:   errCsrfTokenMismatch,
			},
			{
				name:      "expired cookie",
				cookieTok: csrfTestToken(clientID, -time.Hour),
				reqTok:    validTok,
				wantErr:   errCsrfTokenExpired,
			},
			{
				name:      "expired request token",
				cookieTok: validTok,
				reqTok:    csrfTestToken(clientID, -time.Hour),
				wantErr:   errCsrfTokenExpired,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				rec := httptest.NewRecorder()
				body := strings.NewReader(CsrfTokenFormName + "=" + tt.reqTok)
				req := httptest.NewRequest(http.MethodPost, "/someUri", body)
				req.Header.Set(ctHeader, formUrlEncoded)
				if tt.cookieTok != "" {
					req.AddCookie(csrfTestCookie(tt.cookieTok))
				}
				wrappedHandler.ServeHTTP(rec, req)

				res := rec.Result()
				defer res.Body.Close()

				attest.Equal(t, res.StatusCode, http.StatusSeeOther)
				attest.Equal(t, res.Header.Get(ongMiddlewareErrorHeader), tt.wantErr.Error())
				attest.Zero(t, res.Header.Get(tokenHeader))
			})
		}
	})

	t.Run("GET requests re-use clientID of valid cookie", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := csrf(someCsrfHandler(msg), tst.SecretKey(), config.DefaultCsrfCookieDuration)

		clientID := id.Random(csrfBytesTokenLength)

		for _, tc := range []struct {
			cookieTok string
			sameID    bool
		}{
			{cookieTok: csrfTestToken(clientID, config.DefaultCsrfCookieDuration), sameID: true},
			{cookieTok: csrfTestToken(clientID, -time.Hour), sameID: false},
			{cookieTok: id.Random(2 * csrfBytesTokenLength), sameID: false},
		} {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			req.AddCookie(csrfTestCookie(tc.cookieTok))
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()
			attest.Equal(t, res.StatusCode, http.StatusOK)

			gotID, err := parseCsrfToken(res.Header.Get(CsrfHeader))
			attest.Ok(t, err)
			if tc.sameID {
				attest.Equal(t, gotID, clientID)
			} else {
				attest.NotEqual(t, gotID, clientID)
			}
		}
	})

	// concurrency safe
	t.Run("POST requests with valid token from mutiple tabs", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		// for this concurrency test, we have to re-use the same wrappedHandler
		// so that state is shared and thus we can see if there is any state which is not handled correctly.
		wrappedHandler := csrf(someCsrfHandler(msg), tst.SecretKey(), config.DefaultCsrfCookieDuration)

		// Each tab makes a GET request. Each tab gets a different token, and the cookie is updated each time.
		tabTokens := []string{}
		cookieTok := ""
		for range 5 {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			if cookieTok != "" {
				req.AddCookie(csrfTestCookie(cookieTok))
			}
			wrappedHandler.ServeHTTP(rec, req)
			res := rec.Result()
			defer res.Body.Close()
			attest.Equal(t, res.StatusCode, http.StatusOK)

			cookieTok = res.Cookies()[0].Value
			tabTokens = append(tabTokens, res.Header.Get(CsrfHeader))
		}
		attest.Equal(t, len(slices.Compact(slices.Clone(tabTokens))), len(tabTokens))

		runTestPerTab := func(tabTok string) {
			// make POST request using the token of the tab, and the latest cookie.
			rec := httptest.NewRecorder()
			body := strings.NewReader(CsrfTokenFormName + "=" + tabTok)
			req := httptest.NewRequest(http.MethodPost, "/someUri", body)
			req.Header.Set(ctHeader, formUrlEncoded)
			req.AddCookie(csrfTestCookie(cookieTok))
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)
			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), msg)
			attest.Equal(t, len(res.Cookies()), 1)
			attest.Equal(t, res.Header.Get(CsrfHeader), res.Header.Get(tokenHeader))
		}

		wg := &sync.WaitGroup{}
		for _, tabTok := range tabTokens {
			wg.Go(func() {
				runTestPerTab(tabTok)
			})
		}
		wg.Wait()
	})
}
