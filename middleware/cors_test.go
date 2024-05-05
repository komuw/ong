package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/komuw/ong/config"
	"go.akshayshah.org/attest"
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
		wrappedHandler := cors(someCorsHandler(msg), nil, nil, nil, false, config.DefaultCorsCacheDuration)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodOptions, "/someUri", nil)
		req.Header.Add(acrmHeader, "is-set") // preflight request header set
		req.Header.Add(originHeader, "localhost")
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusNoContent)
		attest.Equal(t, string(rb), "") // someCorsHandler is NOT called.
	})

	t.Run("appropriate headers set", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := cors(someCorsHandler(msg), []string{"*"}, []string{"*"}, []string{"*"}, false, config.DefaultCorsCacheDuration)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodOptions, "/someUri", nil)
		req.Header.Add(acrmHeader, http.MethodGet) // preflight request header set
		req.Header.Add(originHeader, "localhost")
		req.Header.Add(acrhHeader, "HEAD, POST")
		req.Header.Add(originHeader, "http://example.com")
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusNoContent)
		attest.Equal(t, string(rb), "") // someCorsHandler is NOT called.
		// if this header was set, then the preflight request succeeded
		gotSucess := res.Header.Get(acmaHeader) != ""
		attest.Equal(t, gotSucess, true)

		attest.NotZero(t, res.Header.Get(varyHeader))
		attest.NotZero(t, res.Header.Get(acaoHeader))
		attest.NotZero(t, res.Header.Get(acamHeader))
		attest.NotZero(t, res.Header.Get(acahHeader))
		attest.NotZero(t, res.Header.Get(acmaHeader))
	})

	t.Run("http OPTIONS without preflight request header", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := cors(someCorsHandler(msg), nil, nil, nil, false, config.DefaultCorsCacheDuration)
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
			statusCode     int
			content        string
		}{
			{
				name:           "star origins",
				origin:         "http://example.com",
				allowedOrigins: []string{"*"},
				succeed:        true,
				statusCode:     http.StatusNoContent,
				content:        "", // someCorsHandler is NOT called.
			},
			{
				name:           "origin not matched",
				origin:         "http://example.com",
				allowedOrigins: []string{"https://example.com", "http://www.hey.com"},
				succeed:        false,
				statusCode:     http.StatusNoContent,
				content:        "", // someCorsHandler is NOT called.
			},
			{
				name:           "origin matched",
				origin:         "http://www.example.com",
				allowedOrigins: []string{"http://www.example.com", "http://www.hey.com"},
				succeed:        true,
				statusCode:     http.StatusNoContent,
				content:        "", // someCorsHandler is NOT called.
			},
			{
				name:           "empty origin",
				origin:         "",
				allowedOrigins: []string{"*"},
				succeed:        false,
				statusCode:     http.StatusOK,                      // since origin is empty, it is not treated as a pre-flight request.
				content:        "hello there from someCorsHandler", // someCorsHandler is called.
			},
		}

		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				msg := "hello there from someCorsHandler"
				wrappedHandler := cors(someCorsHandler(msg), tt.allowedOrigins, []string{"*"}, []string{"*"}, false, config.DefaultCorsCacheDuration)
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodOptions, "/someUri", nil)
				req.Header.Add(acrmHeader, "is-set") // preflight request header set
				req.Header.Add(originHeader, tt.origin)
				wrappedHandler.ServeHTTP(rec, req)

				res := rec.Result()
				defer res.Body.Close()

				rb, err := io.ReadAll(res.Body)
				attest.Ok(t, err)

				attest.Equal(t, res.StatusCode, tt.statusCode)
				attest.Equal(t, string(rb), tt.content)

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

				wrappedHandler := cors(someCorsHandler(msg), []string{"*"}, tt.allowedMethods, []string{"*"}, false, config.DefaultCorsCacheDuration)
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
				wrappedHandler := cors(someCorsHandler(msg), []string{"*"}, []string{"*"}, tt.allowedHeaders, false, config.DefaultCorsCacheDuration)
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

func TestCorsActualRequest(t *testing.T) {
	t.Parallel()

	t.Run("actualRequest success", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := cors(someCorsHandler(msg), nil, nil, nil, false, config.DefaultCorsCacheDuration)
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

	t.Run("appropriate headers set", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := cors(someCorsHandler(msg), []string{"*"}, []string{"*"}, []string{"*"}, false, config.DefaultCorsCacheDuration)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.Header.Add(originHeader, "http://example.com")
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)

		attest.NotZero(t, res.Header.Get(varyHeader))
		attest.NotZero(t, res.Header.Get(acaoHeader))
	})

	t.Run("origin", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name           string
			origin         string
			allowedOrigins []string
			succeed        bool
			statusCode     int
		}{
			{
				name:           "empty origin",
				origin:         "",
				allowedOrigins: []string{"*"},
				succeed:        false,
				statusCode:     http.StatusOK,
			},
			{
				name:           "star origins",
				origin:         "http://example.com",
				allowedOrigins: []string{"*"},
				succeed:        true,
				statusCode:     http.StatusOK,
			},
			{
				name:           "origin not matched",
				origin:         "http://example.com",
				allowedOrigins: []string{"https://example.com", "http://www.hey.com"},
				succeed:        false,
				statusCode:     http.StatusOK,
			},
			{
				name:           "origin matched",
				origin:         "http://www.example.com",
				allowedOrigins: []string{"http://www.example.com", "http://www.hey.com"},
				succeed:        true,
				statusCode:     http.StatusOK,
			},
		}

		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				msg := "hello"
				wrappedHandler := cors(someCorsHandler(msg), tt.allowedOrigins, []string{"*"}, []string{"*"}, false, config.DefaultCorsCacheDuration)
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
				req.Header.Add(originHeader, tt.origin)
				wrappedHandler.ServeHTTP(rec, req)

				res := rec.Result()
				defer res.Body.Close()

				rb, err := io.ReadAll(res.Body)
				attest.Ok(t, err)

				attest.Equal(t, res.StatusCode, tt.statusCode)
				attest.Equal(t, string(rb), msg)
				attest.NotZero(t, res.Header.Get(varyHeader))

				// if this header was set, then the actual request succeeded
				gotSucess := res.Header.Get(acaoHeader) != ""
				attest.Equal(t, gotSucess, tt.succeed)
			})
		}
	})

	t.Run("method", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name           string
			method         string
			allowedMethods []string
			succeed        bool
		}{
			{
				name:           "empty method",
				method:         "",
				allowedMethods: []string{"*"},
				succeed:        true,
			},
			{
				name:           "star methods",
				method:         http.MethodDelete,
				allowedMethods: []string{"*"},
				succeed:        true,
			},
			{
				name:           "method not matched",
				method:         http.MethodDelete,
				allowedMethods: []string{http.MethodGet, http.MethodPatch},
				succeed:        false,
			},
			{
				name:           "method matched",
				method:         http.MethodDelete,
				allowedMethods: []string{http.MethodGet, http.MethodDelete, http.MethodPatch},
				succeed:        true,
			},
		}

		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				msg := "hello"
				wrappedHandler := cors(someCorsHandler(msg), []string{"*"}, tt.allowedMethods, []string{"*"}, false, config.DefaultCorsCacheDuration)
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(tt.method, "/someUri", nil)
				req.Header.Add(originHeader, "http://some-origin.com")
				wrappedHandler.ServeHTTP(rec, req)

				res := rec.Result()
				defer res.Body.Close()

				rb, err := io.ReadAll(res.Body)
				attest.Ok(t, err)

				attest.Equal(t, res.StatusCode, http.StatusOK)
				attest.Equal(t, string(rb), msg)
				attest.NotZero(t, res.Header.Get(varyHeader))

				// if this header was set, then the actual request succeeded
				gotSucess := res.Header.Get(acaoHeader) != ""
				attest.Equal(t, gotSucess, tt.succeed)
			})
		}
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		// for this concurrency test, we have to re-use the same wrappedHandler
		// so that state is shared and thus we can see if there is any state which is not handled correctly.
		wrappedHandler := cors(someCorsHandler(msg), nil, nil, nil, false, config.DefaultCorsCacheDuration)

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

func TestValidateAllowedOrigins(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		allowedOrigins []string
		succeeds       bool
		errMsg         string
	}{
		{
			name:           "okay",
			allowedOrigins: []string{"http://a.com"},
			succeeds:       true,
			errMsg:         "",
		},
		{
			name:           "has slash url path",
			allowedOrigins: []string{"http://b.com/"},
			succeeds:       false,
			errMsg:         "contains url path",
		},
		{
			name:           "has url path",
			allowedOrigins: []string{"https://c.com/hello"},
			succeeds:       false,
			errMsg:         "contains url path",
		},
		{
			name:           "okay with port",
			allowedOrigins: []string{"https://d.com:8888"},
			succeeds:       true,
			errMsg:         "",
		},
		{
			name:           "custom scheme okay",
			allowedOrigins: []string{"hzzs://e.com"},
			succeeds:       true,
			errMsg:         "",
		},
		{
			name:           "missing scheme",
			allowedOrigins: []string{"f.com"},
			succeeds:       false,
			errMsg:         "bad scheme",
		},
		{
			name:           "wildcard with others",
			allowedOrigins: []string{"https://g.com", "*"},
			succeeds:       false,
			errMsg:         "single wildcard used together with others",
		},
		{
			name:           "multiple wildcard",
			allowedOrigins: []string{"http://*h*.com"},
			succeeds:       false,
			errMsg:         "contains more than one wildcard",
		},
		{
			name:           "wildcard not prefixed to host",
			allowedOrigins: []string{"http://i*.com"},
			succeeds:       false,
			errMsg:         "wildcard not prefixed to host",
		},
		{
			name:           "wildcard is okay",
			allowedOrigins: []string{"http://*j.com"},
			succeeds:       true,
			errMsg:         "",
		},
		{
			name:           "wildcard in different domains",
			allowedOrigins: []string{"http://*k.com", "http://*another.com"},
			succeeds:       true,
			errMsg:         "",
		},
		{
			name:           "one wildcard",
			allowedOrigins: []string{"*"},
			succeeds:       true,
			errMsg:         "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateAllowedOrigins(tt.allowedOrigins)
			if tt.succeeds {
				attest.Ok(t, err)
			} else {
				attest.Error(t, err)
				attest.Subsequence(t, err.Error(), tt.errMsg)
			}
		})
	}
}

func TestValidateAllowCredentials(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		allowCredentials bool
		allowedOrigins   []string
		allowedMethods   []string
		allowedHeaders   []string
		succeeds         bool
		errMsg           string
	}{
		{
			name:             "one wildcard origin and credentials",
			allowCredentials: true,
			allowedOrigins:   []string{"*"},
			allowedMethods:   nil,
			allowedHeaders:   nil,
			succeeds:         false,
			errMsg:           "allowCredentials used together with wildcard",
		},
		{
			name:             "credentials no wildcard origin",
			allowCredentials: true,
			allowedOrigins:   []string{"http://example.com"},
			allowedMethods:   nil,
			allowedHeaders:   nil,
			succeeds:         true,
			errMsg:           "",
		},
		{
			name:             "one wildcard method and credentials",
			allowCredentials: true,
			allowedOrigins:   nil,
			allowedMethods:   []string{"*"},
			allowedHeaders:   nil,
			succeeds:         false,
			errMsg:           "allowCredentials used together with wildcard",
		},
		{
			name:             "one wildcard header and credentials",
			allowCredentials: true,
			allowedOrigins:   nil,
			allowedMethods:   nil,
			allowedHeaders:   []string{"*"},
			succeeds:         false,
			errMsg:           "allowCredentials used together with wildcard",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateAllowCredentials(tt.allowCredentials, tt.allowedOrigins, tt.allowedMethods, tt.allowedHeaders)
			if tt.succeeds {
				attest.Ok(t, err)
			} else {
				attest.Error(t, err)
				attest.Subsequence(t, err.Error(), tt.errMsg)
			}
		})
	}
}
