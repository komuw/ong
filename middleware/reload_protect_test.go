package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/akshayjshah/attest"
)

func someReloadProtectorHandler(msg, expectedFormName, expectedFormValue string) http.HandlerFunc {
	// count is state that is affected by form submission.
	// eg, when a form is submitted; we create a new user.
	count := 0
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			err := r.ParseForm()
			if err != nil {
				panic(err)
			}
			val := r.Form.Get(expectedFormName)
			if val != expectedFormValue {
				panic(fmt.Sprintf("expected = %v got = %v", expectedFormValue, val))
			}

			count = count + 1
			if count > 1 {
				// form re-submission happened
				panic("form re-submission happened")
			}
		}

		fmt.Fprint(w, msg)
	}
}

func TestReloadProtector(t *testing.T) {
	t.Parallel()

	t.Run("middleware succeds", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "localhost"
		expectedFormName := "user_name"
		expectedFormValue := "John Doe"
		wrappedHandler := ReloadProtector(someReloadProtectorHandler(msg, expectedFormName, expectedFormValue), domain)

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

	t.Run("re-submission protected", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "localhost"
		expectedFormName := "user_name"
		expectedFormValue := "John Doe"
		wrappedHandler := ReloadProtector(someReloadProtectorHandler(msg, expectedFormName, expectedFormValue), domain)

		req := httptest.NewRequest(http.MethodPost, "/someUri", nil)
		err := req.ParseForm()
		attest.Ok(t, err)
		req.Form.Add(expectedFormName, expectedFormValue)

		var addedCookie *http.Cookie
		{
			// first form submission
			rec := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			rb, errR := io.ReadAll(res.Body)
			attest.Ok(t, errR)

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), msg)

			attest.Equal(t, len(res.Cookies()), 1)
			attest.Subsequence(t, res.Cookies()[0].Name, reloadProtectCookiePrefix)
			addedCookie = res.Cookies()[0]
		}

		{
			// second form submission
			req.AddCookie(addedCookie)
			rec := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			rb, errR := io.ReadAll(res.Body)
			attest.Ok(t, errR)

			attest.Equal(t, res.StatusCode, http.StatusSeeOther)
			attest.Equal(t, string(rb), "")
			attest.Equal(t, len(res.Cookies()), 0)
		}
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		domain := "localhost"
		expectedFormName := "user_name"
		expectedFormValue := "John Doe"
		wrappedHandler := ReloadProtector(someReloadProtectorHandler(msg, expectedFormName, expectedFormValue), domain)

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
