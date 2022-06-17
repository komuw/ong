package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/akshayjshah/attest"
)

func someCorsHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, msg)
	}
}

func TestCorsPreflight(t *testing.T) {
	t.Parallel()

	t.Run("preflight success", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := Cors(someCorsHandler(msg), nil, nil, nil)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodOptions, "/someUri", nil)
		req.Header.Add(acrmHeader, "is-set") // preflight request header set
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusNoContent)
		attest.Equal(t, string(rb), "") // someCorsHandler is NOT called.
	})

	t.Run("http OPTIONS without preflight request header", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := Cors(someCorsHandler(msg), nil, nil, nil)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodOptions, "/someUri", nil)
		// preflight request header NOT set
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg) // someCorsHandler is called.
	})
}

// func TestCorsActualRequest(t *testing.T) {
// 	t.Parallel()

// 	t.Run("TODO", func(t *testing.T) {
// 		t.Parallel()

// 		msg := "hello"
// 		wrappedHandler := Cors(someCorsHandler(msg))
// 		rec := httptest.NewRecorder()
// 		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
// 		wrappedHandler.ServeHTTP(rec, req)

// 		res := rec.Result()
// 		defer res.Body.Close()

// 		rb, err := io.ReadAll(res.Body)
// 		attest.Ok(t, err)

// 		attest.Equal(t, res.StatusCode, http.StatusOK)
// 		attest.Equal(t, string(rb), msg)
// 	})
// }

func TestIsOriginAllowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		origin         string
		allowedOrigins []string
		allow          bool
		allowAll       bool
	}{
		{
			name:           "nil allowedOrigins",
			origin:         "some-origin",
			allowedOrigins: nil,
			allow:          true,
			allowAll:       true,
		},
		{
			name:           "star allowedOrigins",
			origin:         "some-origin",
			allowedOrigins: []string{"*"},
			allow:          true,
			allowAll:       true,
		},
		{
			name:           "matched allowedOrigins",
			origin:         "http://example.com",
			allowedOrigins: []string{"http://hey.com", "http://example.com"},
			allow:          true,
			allowAll:       false,
		},
		{
			name:   "not matched allowedOrigins",
			origin: "http://example.com",
			// an origin consists of the scheme, domain & port
			allowedOrigins: []string{"https://example.com"},
			allow:          false,
			allowAll:       false,
		},
		{
			name:           "star allowedOrigins is supreme",
			origin:         "http://hey.com",
			allowedOrigins: []string{"https://example.com", "*"},
			allow:          true,
			allowAll:       true,
		},
		{
			name:           "wildcard allowedOrigins",
			origin:         "http://example.com",
			allowedOrigins: []string{"*example.com"},
			allow:          true,
			allowAll:       false,
		},
		{
			name:           "wildcard even in scheme ",
			origin:         "https://www.example.com",
			allowedOrigins: []string{"*example.com"},
			allow:          true,
			allowAll:       false,
		},
		{
			name:           "wildcard subdomain",
			origin:         "https://subdomain.example.com",
			allowedOrigins: []string{"*example.com"},
			allow:          true,
			allowAll:       false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			allowedOrigins, allowedWildcardOrigins := getOrigins(tt.allowedOrigins)
			allow, allowAll := isOriginAllowed(tt.origin, allowedOrigins, allowedWildcardOrigins)
			attest.Equal(t, allow, tt.allow)
			attest.Equal(t, allowAll, tt.allowAll)
		})
	}
}

func TestIsMethodAllowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		method         string
		allowedMethods []string
		allowed        bool
	}{
		{
			name:           "nil allowedMethods",
			method:         http.MethodDelete,
			allowedMethods: nil,
			allowed:        false,
		},
		{
			name:           "nil allowedMethods allows some simple headers",
			method:         http.MethodPost,
			allowedMethods: nil,
			allowed:        true,
		},
		{
			name:           "star allowedMethods",
			method:         http.MethodDelete,
			allowedMethods: []string{"*"},
			allowed:        true,
		},
		{
			name:           "http OPTIONS is always allowed",
			method:         http.MethodOptions,
			allowedMethods: nil,
			allowed:        true,
		},
		{
			name:           "method matched",
			method:         http.MethodConnect,
			allowedMethods: []string{http.MethodConnect, http.MethodDelete},
			allowed:        true,
		},
		{
			name:           "method not matched",
			method:         http.MethodConnect,
			allowedMethods: []string{http.MethodGet, http.MethodDelete},
			allowed:        false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			allowedMethods := getMethods(tt.allowedMethods)
			allowed := isMethodAllowed(tt.method, allowedMethods)
			attest.Equal(t, allowed, tt.allowed)
		})
	}
}

func TestAreHeadersAllowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		allowedHeaders []string
		reqHeader      string
		allowed        bool
	}{
		{
			name:           "nil allowedHeaders",
			allowedHeaders: nil,
			reqHeader:      "X-PINGOTHER, Content-Type",
			allowed:        false,
		},
		{
			name:           "star allowedHeaders",
			allowedHeaders: []string{"*"},
			reqHeader:      "X-PINGOTHER, Content-Type",
			allowed:        true,
		},
		{
			name:           "empty reqHeader",
			allowedHeaders: nil,
			reqHeader:      "",
			allowed:        true,
		},
		{
			name:           "match allowedHeaders",
			allowedHeaders: []string{"Content-Type", "X-PINGOTHER", "X-APP-KEY"},
			reqHeader:      "X-PINGOTHER, Content-Type",
			allowed:        true,
		},
		{
			name:           "not matched allowedHeaders",
			allowedHeaders: []string{"X-PINGOTHER"},
			reqHeader:      "X-API-KEY, Content-Type",
			allowed:        false,
		},
		{
			name:           "allowedHeaders should be a superset of reqHeader",
			allowedHeaders: []string{"X-PINGOTHER"},
			reqHeader:      "X-PINGOTHER, Content-Type",
			allowed:        false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			allowedHeaders := getHeaders(tt.allowedHeaders)
			allowed := areHeadersAllowed(tt.reqHeader, allowedHeaders)
			attest.Equal(t, allowed, tt.allowed)
		})
	}
}
