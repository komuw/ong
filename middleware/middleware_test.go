// Package middleware provides helpful functions that implement some common functionalities in http servers.
// A middleware is a func that returns a http.HandlerFunc
package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

	msg := "hello world"
	errMsg := "not allowed. only allows http"
	tests := []struct {
		name               string
		middleware         func(wrappedHandler http.HandlerFunc, o opts) http.HandlerFunc
		httpMethod         string
		expectedStatusCode int
		expectedMsg        string
	}{
		// All
		{
			name:               "All middleware http GET",
			middleware:         All,
			httpMethod:         http.MethodGet,
			expectedStatusCode: http.StatusOK,
			expectedMsg:        msg,
		},
		{
			name:               "All middleware http TRACE",
			middleware:         All,
			httpMethod:         http.MethodTrace,
			expectedStatusCode: http.StatusOK,
			expectedMsg:        msg,
		},

		// Get
		{
			name:               "Get middleware http GET",
			middleware:         Get,
			httpMethod:         http.MethodGet,
			expectedStatusCode: http.StatusOK,
			expectedMsg:        msg,
		},
		{
			name:               "Get middleware http TRACE",
			middleware:         Get,
			httpMethod:         http.MethodTrace,
			expectedStatusCode: http.StatusMethodNotAllowed,
			expectedMsg:        errMsg,
		},

		// Post
		{
			name:               "Post middleware http POST",
			middleware:         Post,
			httpMethod:         http.MethodPost,
			expectedStatusCode: http.StatusOK,
			expectedMsg:        msg,
		},
		{
			name:               "Post middleware http TRACE",
			middleware:         Post,
			httpMethod:         http.MethodTrace,
			expectedStatusCode: http.StatusMethodNotAllowed,
			expectedMsg:        errMsg,
		},

		// Head
		{
			name:               "Head middleware http HEAD",
			middleware:         Head,
			httpMethod:         http.MethodHead,
			expectedStatusCode: http.StatusOK,
			expectedMsg:        msg,
		},
		{
			name:               "Head middleware http TRACE",
			middleware:         Head,
			httpMethod:         http.MethodTrace,
			expectedStatusCode: http.StatusMethodNotAllowed,
			expectedMsg:        errMsg,
		},

		// Put
		{
			name:               "Put middleware http PUT",
			middleware:         Put,
			httpMethod:         http.MethodPut,
			expectedStatusCode: http.StatusOK,
			expectedMsg:        msg,
		},
		{
			name:               "Put middleware http TRACE",
			middleware:         Put,
			httpMethod:         http.MethodTrace,
			expectedStatusCode: http.StatusMethodNotAllowed,
			expectedMsg:        errMsg,
		},
	}

	csrfToken := ""
	{
		// non-safe http methods(like POST) require a server-known csrf token;
		// otherwise it fails with http 403
		// so here we make a http GET so that we can have a csrf token.
		o := WithOpts("example.com")
		wrappedHandler := All(someMiddlewareTestHandler("hey"), o)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)
		res := rec.Result()
		defer res.Body.Close()
		csrfToken = res.Header.Get(csrfHeader)
		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.NotZero(t, csrfToken)
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			o := WithOpts("example.com")
			wrappedHandler := tt.middleware(someMiddlewareTestHandler(msg), o)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tt.httpMethod, "/someUri", nil)
			req.AddCookie(
				&http.Cookie{
					Name:  csrfCookieName,
					Value: csrfToken,
				})
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t, res.StatusCode, tt.expectedStatusCode)
			attest.True(t, strings.Contains(string(rb), tt.expectedMsg))
		})
	}
}
