package mx

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.akshayshah.org/attest"
)

func TestRouter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		RouteMethod  string
		RoutePattern string

		Method string
		Path   string
		Match  bool
		Params map[string]string
	}{
		// simple path matching
		{
			"GET",
			"/one",
			"GET",
			"/one",
			true,
			nil,
		},
		{
			"GET",
			"/two",
			"GET",
			"/two",
			true,
			nil,
		},
		{
			"GET",
			"/three",
			"GET",
			"/three",
			true,
			nil,
		},
		// methods
		{
			"get",
			"/methodcase",
			"GET",
			"/methodcase",
			true,
			nil,
		},
		{
			"Get",
			"/methodcase",
			"get",
			"/methodcase",
			true,
			nil,
		},
		{
			"GET",
			"/methodcase",
			"get",
			"/methodcase",
			true,
			nil,
		},
		{
			"GET",
			"/method1",
			"POST",
			"/method1",
			true,
			nil,
		},
		{
			"DELETE",
			"/method2",
			"GET",
			"/method2",
			true,
			nil,
		},
		{
			"GET",
			"/method3",
			"PUT",
			"/method3",
			true,
			nil,
		},
		// nested
		{
			"GET",
			"/parent/child/one",
			"GET",
			"/parent/child/one",
			true,
			nil,
		},
		{
			"GET",
			"/parent/child/two",
			"GET",
			"/parent/child/two",
			true,
			nil,
		},
		{
			"GET",
			"/parent/child/three",
			"GET",
			"/parent/child/three",
			true,
			nil,
		},
		// slashes
		{
			"GET",
			"slashes/one",
			"GET",
			"/slashes/one",
			true,
			nil,
		},
		{
			"GET",
			"/slashes/two",
			"GET",
			"slashes/two",
			true,
			nil,
		},
		{
			"GET",
			"slashes/three/",
			"GET",
			"/slashes/three",
			true,
			nil,
		},
		{
			"GET",
			"/slashes/four",
			"GET",
			"slashes/four/",
			true,
			nil,
		},
		// prefix
		{
			"GET",
			"/prefix/",
			"GET",
			"/prefix/anything/else",
			false,
			nil,
		},
		{
			"GET",
			"/not-prefix",
			"GET",
			"/not-prefix/anything/else",
			false,
			nil,
		},
		// path params
		{
			"GET",
			"/path-param/:id",
			"GET",
			"/path-param/123",
			true,
			map[string]string{"id": "123"},
		},
		{
			"GET",
			"/path-params/:era/:group/:member",
			"GET",
			"/path-params/60s/beatles/lennon",
			true,
			map[string]string{
				"era":    "60s",
				"group":  "beatles",
				"member": "lennon",
			},
		},
		{
			"GET",
			"/path-params/:era/:group/:member",
			"GET",
			"/path-params/60s/beatles/lennon/",
			true,
			map[string]string{
				"era":    "60s",
				"group":  "beatles",
				"member": "lennon",
			},
		},
		{
			"GET",
			"/path-params-prefix/:era/:group/:member/",
			"GET",
			"/path-params-prefix/60s/beatles/lennon/yoko",
			false,
			nil,
		},
		// misc no matches
		{
			"GET",
			"/not/enough",
			"GET",
			"/not/enough/items",
			false,
			nil,
		},
		{
			"GET",
			"/not/enough/items",
			"GET",
			"/not/enough",
			false,
			nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		name := fmt.Sprintf("%s-%s-%s-%s-%v",
			tt.RouteMethod,
			tt.RoutePattern,
			tt.Method,
			tt.Path,
			tt.Match,
		)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r := newRouter(nil)
			match := false
			var ctx context.Context

			err := r.handle(tt.RouteMethod, tt.RoutePattern, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				match = true
				ctx = r.Context()
			}))
			attest.Ok(t, err)

			req, err := http.NewRequest(tt.Method, tt.Path, nil)
			attest.Ok(t, err)
			w := httptest.NewRecorder()
			r.serveHTTP(w, req)
			attest.Equal(
				t,
				match,
				tt.Match,
				attest.Sprintf("expected match %v but was %v: %s %s", tt.Match, match, tt.Method, tt.Path),
			)
			if len(tt.Params) > 0 {
				for expK, expV := range tt.Params {
					// check using helper
					actualValStr := Param(ctx, expK)
					attest.Equal(
						t,
						actualValStr,
						expV,
						attest.Sprintf("Param: context value %s expected \"%s\" but was \"%s\"", expK, expV, actualValStr),
					)
				}
			}
		})
	}
}

func TestMultipleRoutesDifferentMethods(t *testing.T) {
	t.Parallel()

	r := newRouter(nil)
	var match string
	err := r.handle(MethodAll, "/path", nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		match = r.Method
	}))
	attest.Ok(t, err)

	req, err := http.NewRequest(http.MethodGet, "/path", nil)
	attest.Ok(t, err)
	r.serveHTTP(httptest.NewRecorder(), req)
	attest.Equal(t, match, "GET")

	req, err = http.NewRequest(http.MethodDelete, "/path", nil)
	attest.Ok(t, err)
	r.serveHTTP(httptest.NewRecorder(), req)
	attest.Equal(t, match, "DELETE")

	req, err = http.NewRequest(http.MethodPost, "/path", nil)
	attest.Ok(t, err)
	r.serveHTTP(httptest.NewRecorder(), req)
	attest.Equal(t, match, "POST")
}

func firstRoute(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, msg)
	}
}

func secondRoute(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, msg)
	}
}

// TODO: remove
// func TestConflicts(t *testing.T) {
// 	t.Parallel()

// 	t.Run("conflicts detected", func(t *testing.T) {
// 		t.Parallel()
// 		r := newRouter(nil)

// 		msg1 := "firstRoute"
// 		msg2 := "secondRoute"
// 		err := r.handle(http.MethodGet, "/post/create", firstRoute(msg1), firstRoute(msg1))
// 		attest.Ok(t, err)

// 		// This one returns with a conflict message.
// 		errH := r.handle(http.MethodGet, "/post/:id", secondRoute(msg2), secondRoute(msg2))
// 		attest.Error(t, errH)

// 		rec := httptest.NewRecorder()
// 		req := httptest.NewRequest(http.MethodGet, "/post/create", nil)
// 		r.serveHTTP(rec, req)

// 		res := rec.Result()
// 		defer res.Body.Close()

// 		rb, err := io.ReadAll(res.Body)
// 		attest.Ok(t, err)

// 		attest.Equal(t, res.StatusCode, http.StatusOK)
// 		attest.Equal(t, string(rb), msg1)
// 	})

// 	t.Run("different http methods same path conflicts detected", func(t *testing.T) {
// 		t.Parallel()
// 		r := newRouter(nil)

// 		msg1 := "firstRoute"
// 		msg2 := "secondRoute"
// 		err := r.handle(http.MethodGet, "/post", firstRoute(msg1), firstRoute(msg1))
// 		attest.Ok(t, err)

// 		// This one returns with a conflict message.
// 		errH := r.handle(http.MethodGet, "/post/", secondRoute(msg2), secondRoute(msg2))
// 		attest.Error(t, errH)

// 		// This one returns with a conflict message.
// 		errB := r.handle(http.MethodDelete, "post/", secondRoute(msg2), secondRoute(msg2))
// 		attest.Error(t, errB)

// 		// This one returns with a conflict message.
// 		errC := r.handle(http.MethodPut, "post", secondRoute(msg2), secondRoute(msg2))
// 		attest.Error(t, errC)
// 	})

// 	t.Run("no conflict", func(t *testing.T) {
// 		t.Parallel()
// 		r := newRouter(nil)

// 		msg1 := "firstRoute-one"
// 		msg2 := "secondRoute-two"
// 		err := r.handle(http.MethodGet, "/w00tw00t.at.blackhats.romanian.anti-sec:)", firstRoute(msg1), firstRoute(msg1))
// 		attest.Ok(t, err)

// 		// This one should not conflict.
// 		errH := r.handle(http.MethodGet, "/index.php", secondRoute(msg2), secondRoute(msg2))
// 		attest.Ok(t, errH)
// 	})

// 	t.Run("http MethodAll conflicts with all other methods", func(t *testing.T) {
// 		t.Parallel()
// 		r := newRouter(nil)

// 		msg1 := "firstRoute"
// 		msg2 := "secondRoute"
// 		err := r.handle(http.MethodGet, "/post", firstRoute(msg1), firstRoute(msg1))
// 		attest.Ok(t, err)

// 		// This one returns with a conflict message.
// 		errB := r.handle(MethodAll, "post/", secondRoute(msg2), secondRoute(msg2))
// 		attest.Error(t, errB)
// 	})
// }

func TestNotFound(t *testing.T) {
	t.Parallel()

	t.Run("path exists", func(t *testing.T) {
		t.Parallel()

		r := newRouter(nil)
		var match string
		err := r.handle(MethodAll, "/path", nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			match = r.Method
		}))
		attest.Ok(t, err)

		req, err := http.NewRequest(http.MethodGet, "/path", nil)
		attest.Ok(t, err)
		rec := httptest.NewRecorder()
		r.serveHTTP(rec, req)
		attest.Equal(t, match, "GET")
		res := rec.Result()
		defer res.Body.Close()
		attest.Equal(t, res.StatusCode, http.StatusOK)
	})

	t.Run("path not exists", func(t *testing.T) {
		t.Parallel()

		r := newRouter(nil)
		var match string
		err := r.handle(MethodAll, "/path", nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			match = r.Method
		}))
		attest.Ok(t, err)

		req, err := http.NewRequest(http.MethodGet, "/not-found-path", nil)
		attest.Ok(t, err)
		rec := httptest.NewRecorder()
		r.serveHTTP(rec, req)
		attest.Equal(t, match, "")
		res := rec.Result()
		defer res.Body.Close()
		attest.Equal(t, res.StatusCode, http.StatusNotFound)
	})

	t.Run("custom notFoundHandler", func(t *testing.T) {
		t.Parallel()

		var match string
		notFoundHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			match = "notFoundHandler"
		})

		r := newRouter(notFoundHandler)
		err := r.handle(MethodAll, "/path", nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			match = r.Method
		}))
		attest.Ok(t, err)

		req, err := http.NewRequest(http.MethodGet, "/not-found-path", nil)
		attest.Ok(t, err)
		rec := httptest.NewRecorder()
		r.serveHTTP(rec, req)
		attest.Equal(t, match, "notFoundHandler")
		res := rec.Result()
		defer res.Body.Close()
		attest.Equal(t, res.StatusCode, http.StatusOK)
	})
}
