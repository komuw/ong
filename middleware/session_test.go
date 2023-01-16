package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/akshayjshah/attest"
	"github.com/komuw/ong/cookie"
	"github.com/komuw/ong/sess"
)

func bigMap() map[string]string {
	y := map[string]string{}
	for i := 0; i < 100; i++ {
		k := fmt.Sprintf("key:%d", i)
		v := fmt.Sprintf("val:%d", i)
		y[k] = v
	}
	return y
}

func someSessionHandler(msg, key, value string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess.Set(r, key, value)
		sess.SetM(r, bigMap())
		fmt.Fprint(w, msg)
	}
}

func TestSession(t *testing.T) {
	t.Parallel()

	t.Run("middleware succeds", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		secretKey := "secretKey"
		domain := "localhost"
		key := "name"
		value := "John Doe"
		wrappedHandler := session(someSessionHandler(msg, key, value), secretKey, domain)

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

	t.Run("middleware set succeds", func(t *testing.T) {
		t.Parallel()

		msg := "hello world wide."
		secretKey := "secretKey"
		domain := "localhost"
		key := "name"
		value := "John Doe"
		wrappedHandler := session(someSessionHandler(msg, key, value), secretKey, domain)

		ts := httptest.NewServer(
			wrappedHandler,
		)
		t.Cleanup(func() {
			ts.Close()
		})

		res, err := ts.Client().Get(ts.URL)
		attest.Ok(t, err)
		t.Cleanup(func() {
			res.Body.Close()
		})

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)

		attest.Equal(t, len(res.Cookies()), 1)
		attest.Equal(t, res.Cookies()[0].Name, sess.CookieName)
		attest.NotZero(t, res.Cookies()[0].Value)

		{
			req2 := httptest.NewRequest(http.MethodGet, "/hey-uri", nil)
			// very important to do this assignment, since `cookie.GetEncrypted()` checks for IP mismatch.
			req2.RemoteAddr = ts.Listener.Addr().String()
			req2.AddCookie(&http.Cookie{
				Name:  res.Cookies()[0].Name,
				Value: res.Cookies()[0].Value,
			})

			c, errG := cookie.GetEncrypted(req2, sess.CookieName, secretKey)
			attest.Ok(t, errG)
			attest.Subsequence(t, c.Value, key)
			attest.Subsequence(t, c.Value, value)
		}
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		secretKey := "secretKey"
		domain := "localhost"
		key := "name"
		value := "John Doe"
		wrappedHandler := session(someSessionHandler(msg, key, value), secretKey, domain)

		runhandler := func() {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), msg)
		}

		wg := &sync.WaitGroup{}
		for rN := 0; rN <= 10; rN++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				runhandler()
			}()
		}
		wg.Wait()
	})
}
