package server

import (
	"net/http"
	"strings"

	"github.com/komuw/ong/log"
	"github.com/komuw/ong/middleware"
)

const (
	MethodAll     = "ALL"
	MethodGet     = http.MethodGet
	MethodHead    = http.MethodHead
	MethodPost    = http.MethodPost
	MethodPut     = http.MethodPut
	MethodPatch   = http.MethodPatch
	MethodDelete  = http.MethodDelete
	MethodConnect = http.MethodConnect
	MethodOptions = http.MethodOptions
	MethodTrace   = http.MethodTrace
)

// route relates a uri to its http method and http Handler.
type route struct {
	pattern string
	method  string
	handler http.HandlerFunc
}

// NewRoute creates a new route.
func NewRoute(
	pattern string,
	method string,
	handler http.HandlerFunc,
) route {
	return route{
		pattern: pattern,
		method:  method,
		handler: handler,
	}
}

// Routes is a list of all the route for an application.
type Routes []route

// mux implements server.extendedHandler
type mux struct {
	l      log.Logger
	router *http.ServeMux // some router
}

// NewMux creates a new mux.
func NewMux(l log.Logger, opt middleware.Opts, rts Routes) *mux {
	m := &mux{
		l:      l,
		router: http.NewServeMux(),
	}

	mid := middleware.All //nolint:ineffassign
	for _, rt := range rts {
		switch rt.method {
		case MethodAll:
			mid = middleware.All
		case MethodGet:
			mid = middleware.Get
		case MethodPost:
			mid = middleware.Post
		case MethodHead:
			mid = middleware.Head
		case MethodPut:
			mid = middleware.Put
		case MethodDelete:
			mid = middleware.Delete
		default:
			mid = middleware.All
		}

		m.addPattern(rt.pattern,
			mid(rt.handler, opt),
		)
	}

	return m
}

// ServeHTTP implements a http.Handler
func (m *mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.router.ServeHTTP(w, r)
}

func (m *mux) GetLogger() log.Logger {
	return m.l
}

func (m *mux) addPattern(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	if !strings.HasSuffix(pattern, "/") {
		// this will make the mux send requests for;
		//   - localhost:80/check
		//   - localhost:80/check/
		// to the same handler.
		pattern = pattern + "/"
	}
	if !strings.HasPrefix(pattern, "/") {
		pattern = "/" + pattern
	}
	m.router.HandleFunc(pattern, handler)
}
