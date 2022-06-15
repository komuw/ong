package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/akshayjshah/attest"
	"github.com/rs/xid"
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

		attest.False(t, store.exists("NonExistent"))
		attest.True(t, store.exists(tokens[14]))
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

		r := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		tok := getToken(r)
		attest.Zero(t, tok)
	})

	t.Run("from cookie", func(t *testing.T) {
		t.Parallel()

		want := xid.New().String()
		r := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		r.AddCookie(&http.Cookie{
			Name:     cookieName,
			Value:    want,
			Path:     "/",
			HttpOnly: false, // If true, makes cookie inaccessible to JS. Should be false for csrf cookies.
			Secure:   true,  // https only.
			SameSite: http.SameSiteStrictMode,
		})
		got := getToken(r)
		attest.Equal(t, got, want)
	})

	t.Run("from header", func(t *testing.T) {
		t.Parallel()

		want := xid.New().String()
		r := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		r.Header.Set(csrfHeader, want)
		got := getToken(r)
		attest.Equal(t, got, want)
	})

	t.Run("from form", func(t *testing.T) {
		t.Parallel()

		want := xid.New().String()
		r := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		err := r.ParseForm()
		attest.Ok(t, err)
		r.Form.Add(cookieForm, want)
		got := getToken(r)
		attest.Equal(t, got, want)
	})

	t.Run("cookie takes precedence", func(t *testing.T) {
		t.Parallel()

		cookieToken := xid.New().String()
		headerToken := xid.New().String()
		formToken := xid.New().String()
		r := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		r.AddCookie(&http.Cookie{
			Name:     cookieName,
			Value:    cookieToken,
			Path:     "/",
			HttpOnly: false, // If true, makes cookie inaccessible to JS. Should be false for csrf cookies.
			Secure:   true,  // https only.
			SameSite: http.SameSiteStrictMode,
		})
		r.Header.Set(csrfHeader, headerToken)
		err := r.ParseForm()
		attest.Ok(t, err)
		r.Form.Add(cookieForm, formToken)

		got := getToken(r)
		attest.Equal(t, got, cookieToken)
	})
}
