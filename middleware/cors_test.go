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

	t.Run("origin", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name           string
			origin         string
			allowedOrigins []string
			succeed        bool
		}{
			{
				name:           "empty origin",
				origin:         "",
				allowedOrigins: []string{"*"},
				succeed:        false,
			},
			{
				name:           "star origins",
				origin:         "http:/example.com",
				allowedOrigins: []string{"*"},
				succeed:        true,
			},
			{
				name:           "origin not matched",
				origin:         "http:/example.com",
				allowedOrigins: []string{"https:/example.com", "http://www.hey.com"},
				succeed:        false,
			},
			{
				name:           "origin matched",
				origin:         "http:/www.example.com",
				allowedOrigins: []string{"http:/www.example.com", "http://www.hey.com"},
				succeed:        true,
			},
		}

		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				msg := "hello"
				wrappedHandler := Cors(someCorsHandler(msg), tt.allowedOrigins, []string{"*"}, []string{"*"})
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodOptions, "/someUri", nil)
				req.Header.Add(acrmHeader, "is-set") // preflight request header set
				req.Header.Add(originHeader, tt.origin)
				wrappedHandler.ServeHTTP(rec, req)

				res := rec.Result()
				defer res.Body.Close()

				rb, err := io.ReadAll(res.Body)
				attest.Ok(t, err)

				attest.Equal(t, res.StatusCode, http.StatusNoContent)
				attest.Equal(t, string(rb), "") // someCorsHandler is NOT called.

				// if this header was set, then the preflight request succeeded
				gotSucess := res.Header.Get(acmaHeader) != ""
				attest.Equal(t, gotSucess, tt.succeed)
			})
		}
	})

	t.Run("method", func(t *testing.T) {
		t.Parallel()
		msg := "hello"
		tests := []struct {
			name           string
			method         string
			allowedMethods []string
			succeed        bool
			statusCode     int
			resContent     string
		}{
			{
				// for empty method, it is not treated as a prefight request and instead treated as an actual request.
				name:           "empty method",
				method:         "",
				allowedMethods: []string{"*"},
				succeed:        false,
				statusCode:     http.StatusOK,
				resContent:     msg,
			},
			{
				name:           "star methods",
				method:         http.MethodDelete,
				allowedMethods: []string{"*"},
				succeed:        true,
				statusCode:     http.StatusNoContent,
				resContent:     "",
			},
			{
				name:           "method not matched",
				method:         http.MethodDelete,
				allowedMethods: []string{http.MethodGet, http.MethodPatch},
				succeed:        false,
				statusCode:     http.StatusNoContent,
				resContent:     "",
			},
			{
				name:           "method matched",
				method:         http.MethodDelete,
				allowedMethods: []string{http.MethodGet, http.MethodDelete, http.MethodPatch},
				succeed:        true,
				statusCode:     http.StatusNoContent,
				resContent:     "",
			},
		}

		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				wrappedHandler := Cors(someCorsHandler(msg), []string{"*"}, tt.allowedMethods, []string{"*"})
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodOptions, "/someUri", nil)
				req.Header.Add(originHeader, "http://some-origin.com")
				req.Header.Add(acrmHeader, tt.method)
				wrappedHandler.ServeHTTP(rec, req)

				res := rec.Result()
				defer res.Body.Close()

				rb, err := io.ReadAll(res.Body)
				attest.Ok(t, err)

				attest.Equal(t, res.StatusCode, tt.statusCode)
				attest.Equal(t, string(rb), tt.resContent)

				// if this header was set, then the preflight request succeeded
				gotSucess := res.Header.Get(acmaHeader) != ""
				attest.Equal(t, gotSucess, tt.succeed)
			})
		}
	})

	t.Run("header", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name           string
			header         string
			allowedHeaders []string
			succeed        bool
		}{
			{
				name:           "empty header",
				header:         "",
				allowedHeaders: []string{"Content-Type"},
				succeed:        true,
			},
			{
				name:           "star header",
				header:         "API-KEY",
				allowedHeaders: []string{"*"},
				succeed:        true,
			},
			{
				name:           "header not matched",
				header:         "API-KEY",
				allowedHeaders: []string{"Content-Type", "Accept"},
				succeed:        false,
			},
			{
				name:           "header matched",
				header:         "API-KEY",
				allowedHeaders: []string{"Content-Type", "API-KEY", "Accept"},
				succeed:        true,
			},
		}

		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				msg := "hello"
				wrappedHandler := Cors(someCorsHandler(msg), []string{"*"}, []string{"*"}, tt.allowedHeaders)
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodOptions, "/someUri", nil)
				req.Header.Add(acrmHeader, "is-set") // preflight request header set
				req.Header.Add(originHeader, "http://some-origin.com")
				req.Header.Add(acrhHeader, tt.header)
				wrappedHandler.ServeHTTP(rec, req)

				res := rec.Result()
				defer res.Body.Close()

				rb, err := io.ReadAll(res.Body)
				attest.Ok(t, err)

				attest.Equal(t, res.StatusCode, http.StatusNoContent)
				attest.Equal(t, string(rb), "") // someCorsHandler is NOT called.

				// if this header was set, then the preflight request succeeded
				gotSucess := res.Header.Get(acmaHeader) != ""
				attest.Equal(t, gotSucess, tt.succeed)
			})
		}
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
