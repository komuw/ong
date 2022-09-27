package mux

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/akshayjshah/attest"
)

var tests = []struct {
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
		false,
		nil,
	},
	{
		"DELETE",
		"/method2",
		"GET",
		"/method2",
		false,
		nil,
	},
	{
		"GET",
		"/method3",
		"PUT",
		"/method3",
		false,
		nil,
	},
	// all methods
	{
		"*",
		"/all-methods",
		"GET",
		"/all-methods",
		true,
		nil,
	},
	{
		"*",
		"/all-methods",
		"POST",
		"/all-methods",
		true,
		nil,
	},
	{
		"*",
		"/all-methods",
		"PUT",
		"/all-methods",
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

func TestWay(t *testing.T) {
	t.Parallel()

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

			r := NewRouter()
			match := false
			var ctx context.Context
			r.Handle(tt.RouteMethod, tt.RoutePattern, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				match = true
				ctx = r.Context()
			}))
			req, err := http.NewRequest(tt.Method, tt.Path, nil)
			if err != nil {
				t.Errorf("NewRequest: %s", err)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if match != tt.Match {
				t.Errorf("expected match %v but was %v: %s %s", tt.Match, match, tt.Method, tt.Path)
			}
			if len(tt.Params) > 0 {
				for expK, expV := range tt.Params {
					// check using helper
					actualValStr := Param(ctx, expK)
					if actualValStr != expV {
						t.Errorf("Param: context value %s expected \"%s\" but was \"%s\"", expK, expV, actualValStr)
					}
				}
			}
		})
	}
}

func TestMultipleRoutesDifferentMethods(t *testing.T) {
	t.Parallel()

	r := NewRouter()
	var match string
	r.Handle(http.MethodGet, "/route", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		match = "GET /route"
	}))
	r.Handle(http.MethodDelete, "/route", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		match = "DELETE /route"
	}))
	r.Handle(http.MethodPost, "/route", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		match = "POST /route"
	}))

	req, err := http.NewRequest(http.MethodGet, "/route", nil)
	if err != nil {
		t.Errorf("NewRequest: %s", err)
	}
	r.ServeHTTP(httptest.NewRecorder(), req)
	if match != "GET /route" {
		t.Errorf("unexpected: %s", match)
	}

	req, err = http.NewRequest(http.MethodDelete, "/route", nil)
	if err != nil {
		t.Errorf("NewRequest: %s", err)
	}
	r.ServeHTTP(httptest.NewRecorder(), req)
	if match != "DELETE /route" {
		t.Errorf("unexpected: %s", match)
	}

	req, err = http.NewRequest(http.MethodPost, "/route", nil)
	if err != nil {
		t.Errorf("NewRequest: %s", err)
	}
	r.ServeHTTP(httptest.NewRecorder(), req)
	if match != "POST /route" {
		t.Errorf("unexpected: %s", match)
	}
}

func TestCool(t *testing.T) {
	t.Parallel()

	r := NewRouter()
	var match string

	r.Handle(http.MethodGet, "/post/create", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		match = "GET /post/create"
	}))

	r.Handle(http.MethodGet, "/post/:id", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		match = "GET /post/:id"
	}))

	fmt.Println("\n\t r.routes: ", r.routes)

	req, err := http.NewRequest(http.MethodGet, "/post/create", nil)
	if err != nil {
		t.Errorf("NewRequest: %s", err)
	}
	r.ServeHTTP(httptest.NewRecorder(), req)

	expected := "GET /post/create"

	attest.Equal(t, match, expected)
}
