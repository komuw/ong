package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"go.akshayshah.org/attest"
)

func protectedHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, msg)
	}
}

func TestBasicAuth(t *testing.T) {
	t.Parallel()

	{
		// small passwd panics.
		attest.Panics(t, func() {
			BasicAuth(protectedHandler("hello"), "user", strings.Repeat("a", (minPasswdSize-3)))
		},
		)
	}

	msg := "hello"
	user := "some-user"
	passwd := "some-long-passwd"
	wrappedHandler := BasicAuth(protectedHandler(msg), user, passwd)

	tests := []struct {
		name     string
		user     string
		passwd   string
		wantCode int
	}{
		{
			name:     "no credentials",
			user:     "",
			passwd:   "",
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "user not match",
			user:     "fakeUser",
			passwd:   passwd,
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "passwd not match",
			user:     user,
			passwd:   "fakePasswd",
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "success",
			user:     user,
			passwd:   passwd,
			wantCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			if tt.user != "" || tt.passwd != "" {
				req.SetBasicAuth(tt.user, tt.passwd)
			}
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			attest.True(t, len(passwd) >= minPasswdSize)
			attest.Equal(t, res.StatusCode, tt.wantCode)
		})
	}

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		// for this concurrency test, we have to re-use the same newWrappedHandler
		// so that state is shared and thus we can see if there is any state which is not handled correctly.
		newWrappedHandler := BasicAuth(protectedHandler(msg), user, passwd)

		runhandler := func() {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			req.SetBasicAuth(user, passwd)
			newWrappedHandler.ServeHTTP(rec, req)

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
