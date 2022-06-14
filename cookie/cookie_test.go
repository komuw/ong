package cookie

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/akshayjshah/attest"
)

func setHandler(name, value, domain string, mAge time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		Set(w, name, value, domain, mAge, true)
		fmt.Fprint(w, "hello")
	}
}

func TestSet(t *testing.T) {
	t.Parallel()

	t.Run("set succeds", func(t *testing.T) {
		t.Parallel()

		name := "logId"
		value := "skmHajue8k"
		domain := "localhost"
		mAge := 1 * time.Minute
		handler := setHandler(name, value, domain, mAge)

		rec := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		handler.ServeHTTP(rec, r)

		res := rec.Result()
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, len(res.Cookies()), 1)
		attest.Equal(t, res.Cookies()[0].Name, name)

		cookie := res.Cookies()[0]
		now := time.Now()

		attest.True(t, cookie.MaxAge >= 1)
		attest.True(t, cookie.Expires.Sub(now) > 1)
	})

	t.Run("session cookie", func(t *testing.T) {
		t.Parallel()

		name := "logId"
		value := "skmHajue8k"
		domain := "localhost"
		mAge := 0 * time.Minute
		handler := setHandler(name, value, domain, mAge)

		rec := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		handler.ServeHTTP(rec, r)

		res := rec.Result()
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, len(res.Cookies()), 1)
		attest.Equal(t, res.Cookies()[0].Name, name)

		cookie := res.Cookies()[0]
		attest.Equal(t, cookie.MaxAge, 0)
		attest.Equal(t, cookie.Expires, time.Time{})
	})
}

func deleteHandler(name, value, domain string, mAge time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		Set(w, name, value, domain, mAge, true)
		Delete(w, name, domain)
		fmt.Fprint(w, "hello")
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()

	t.Run("delete", func(t *testing.T) {
		t.Parallel()

		name := "logId"
		domain := "localhost"
		value := "skmHajue8k"
		mAge := 1 * time.Minute
		rec := httptest.NewRecorder()
		handler := deleteHandler(name, value, domain, mAge)

		r := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		handler.ServeHTTP(rec, r)
		res := rec.Result()
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, len(res.Cookies()), 2) // deleting cookies is done by appending to existing cookies.

		cookie := res.Cookies()[1]
		attest.True(t, cookie.MaxAge < 0)
	})
}
