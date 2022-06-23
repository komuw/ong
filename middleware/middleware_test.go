// Package middleware provides helpful functions that implement some common functionalities in http servers.
// A middleware is a func that returns a http.HandlerFunc
package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/akshayjshah/attest"
)

func someMiddlewareTestHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, msg)
	}
}

func TestAllMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		middleware func(wrappedHandler http.HandlerFunc, o opts) http.HandlerFunc
		httpMethod string
		expected   int
	}{
		{
			name:       "All middleware http GET",
			middleware: All,
			httpMethod: http.MethodGet,
			expected:   http.StatusOK,
		},
		{
			name:       "All middleware http TRACE",
			middleware: All,
			httpMethod: http.MethodTrace,
			expected:   http.StatusOK,
		},

		{
			name:       "Get middleware http GET",
			middleware: Get,
			httpMethod: http.MethodGet,
			expected:   http.StatusOK,
		},
		{
			name:       "Get middleware http TRACE",
			middleware: Get,
			httpMethod: http.MethodTrace,
			expected:   http.StatusOK,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			msg := "hello world"
			opts := WithOpts("example.com")
			wrappedHandler := All(someMiddlewareTestHandler(msg), opts)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tt.httpMethod, "/someUri", nil)
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), msg)
		})
	}
}
