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

func somePathTraversalHandler(msg string) http.HandlerFunc {
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

func TestPathTraversal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     string
		expected string
	}{
		{path: "/", expected: "/"},
		{path: "/someUri", expected: "/someUri"},
		{path: "../../etc", expected: "/etc"},
		{path: "../../hashSlashAtEnd/", expected: "/hashSlashAtEnd/"},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(fmt.Sprintf("pathTraversal. url=%s", tt.path), func(t *testing.T) {
			t.Parallel()

			if !strings.HasPrefix(tt.path, "/") {
				// dont use url.JoinPath(), since it cleans urls for you.
				tt.path = "/" + tt.path
			}

			msg := "hello world"
			wrappedHandler := pathTraversal(somePathTraversalHandler(msg))

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)
			defer res.Body.Close()

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), msg)
			attest.Equal(t, req.URL.Path, tt.expected)
		})
	}

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		msg := "hello world"
		// for this concurrency test, we have to re-use the same wrappedHandler
		// so that state is shared and thus we can see if there is any state which is not handled correctly.
		wrappedHandler := pathTraversal(somePathTraversalHandler(msg))

		runhandler := func() {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)
			defer res.Body.Close()

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), msg)
			attest.Equal(t, req.URL.Path, "/someUri")
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
