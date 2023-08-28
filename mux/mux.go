// Package mux implements a HTTP request multiplexer.
package mux

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"net/url"
	"strings"
	"time"

	"github.com/komuw/ong/internal/acme"
	"github.com/komuw/ong/middleware"
)

// Common HTTP methods.
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

// NewRoute creates a new Route.
//
// It panics if handler has already been wrapped with ong/middleware
func NewRoute(
	pattern string,
	method string,
	handler http.Handler,
) Route {
	h := getfunc(handler)
	if strings.Contains(h, "ong/middleware/") &&
		!strings.Contains(h, "ong/middleware.BasicAuth") {
		// BasicAuth is allowed.
		panic("the handler should not be wrapped with ong middleware")
	}

	return Route{
		method:          method,
		pattern:         pattern,
		segments:        pathSegments(pattern),
		originalHandler: handler,
	}
}

// Mux is a HTTP request multiplexer.
//
// It matches the URL of each incoming request against a list of registered
// patterns and calls the handler for the pattern that most closely matches the URL.
// It implements http.Handler
//
// Use [New] to get a valid Mux.
type Mux struct {
	l      *slog.Logger
	router *router // some router
}

// /////////////////////////////////////////////////
func pprofTT() http.HandlerFunc {
	const (
		/*
			The pprof tool supports fetching profles by duration.
			eg; fetch cpu profile for the last 5mins(300sec):
				go tool pprof http://localhost:65079/debug/pprof/profile?seconds=300
			This may fail with an error like:
				http://localhost:65079/debug/pprof/profile?seconds=300: server response: 400 Bad Request - profile duration exceeds server's WriteTimeout
			So we need to be generous with our timeouts. Which is okay since pprof runs in a mux that is not exposed to the internet(localhost)
		*/
		read  = 30 * time.Second
		write = 30 * time.Minute
	)

	pprof := func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		rc := http.NewResponseController(w)

		if err := rc.SetReadDeadline(now.Add(read)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := rc.SetWriteDeadline(now.Add(write)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		path := r.URL.Path
		fmt.Println("\n\t pprofTT called. path: ", path)

		switch path {
		case "/debug/pprof":
			pprof.Index(w, r)
			return
		case "/debug/pprof/cmdline":
			pprof.Cmdline(w, r)
			return
		case "/debug/pprof/profile":
			pprof.Profile(w, r)
			return
		case "/debug/pprof/symbol":
			pprof.Symbol(w, r)
			return
		case "/debug/pprof/trace":
			pprof.Trace(w, r)
			return
		default:
			pprof.Index(w, r)
			return
		}
	}

	return middleware.BasicAuth(
		http.HandlerFunc(pprof),
		"TODO-KJ#4p-Pad64adH",
		"TODO-KJ#4p-Pad64adH",
	)
}

/////////////////////////////////////////////////////

// New returns a HTTP request multiplexer that has the paths in routes.
//
// notFoundHandler is the handler that will be used if a url is not found.
// If it is nil, [http.NotFound] is used instead.
//
// All the paths of an application should be added as part of the routes slice argument.
// Typically, an application should only have one Mux.
//
// It panics with a helpful error message if it detects conflicting routes.
func New(l *slog.Logger, opt middleware.Opts, notFoundHandler http.Handler, routes ...Route) Mux {
	m := Mux{
		l:      l,
		router: newRouter(notFoundHandler),
	}

	mid := middleware.All //nolint:ineffassign
	for _, rt := range routes {
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

		m.addPattern(
			rt.method,
			rt.pattern,
			rt.originalHandler,
			mid(rt.originalHandler, opt),
		)
	}

	{
		// Support for acme certificate manager needs to be added in three places:
		// (a) In http middlewares.
		// (b) In http server.
		// (c) In http multiplexer.
		const acmeChallengeURI = "/.well-known/acme-challenge/:token"
		acmeHandler := acme.Handler(m)
		m.addPattern(
			MethodAll,
			acmeChallengeURI,
			acmeHandler,
			middleware.All(acmeHandler, opt),
		)
	}

	// TODO:

	// TODO: add BasicAuth.
	{
		// This is taken from: https://github.com/golang/go/blob/go1.21.0/src/net/http/pprof/pprof.go#L93-L99
		//
		// m.addPattern(
		// 	MethodAll,
		// 	"/debug/", // TODO: is this one needed?
		// 	http.HandlerFunc(pprof.Index),
		// 	middleware.All(http.HandlerFunc(pprof.Index), opt),
		// )

		h := pprofTT()
		m.addPattern(
			MethodAll,
			"/debug/pprof/:part",
			h,
			middleware.All(h, opt),
		)
		m.addPattern(
			MethodAll,
			"/debug/pprof/",
			h,
			middleware.All(h, opt),
		)

		//
		// m.addPattern(
		// 	MethodAll,
		// 	"/debug/pprof/",
		// 	http.HandlerFunc(pprof.Index),
		// 	middleware.All(http.HandlerFunc(pprof.Index), opt),
		// )
		// m.addPattern(
		// 	MethodAll,
		// 	"/debug/pprof/cmdline",
		// 	http.HandlerFunc(pprof.Cmdline),
		// 	middleware.All(http.HandlerFunc(pprof.Cmdline), opt),
		// )
		// m.addPattern(
		// 	MethodAll,
		// 	"/debug/pprof/profile",
		// 	http.HandlerFunc(pprof.Profile),
		// 	middleware.All(http.HandlerFunc(pprof.Profile), opt),
		// )
		// m.addPattern(
		// 	MethodAll,
		// 	"/debug/pprof/symbol",
		// 	http.HandlerFunc(pprof.Symbol),
		// 	middleware.All(http.HandlerFunc(pprof.Symbol), opt),
		// )
		// m.addPattern(
		// 	MethodAll,
		// 	"/debug/pprof/trace",
		// 	http.HandlerFunc(pprof.Trace),
		// 	middleware.All(http.HandlerFunc(pprof.Trace), opt),
		// )

		// mux.HandleFunc("/debug/pprof/", pprof.Index)
		// mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		// mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		// mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		// mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}

	fmt.Println(m.router.routes)

	return m
}

func (m Mux) addPattern(method, pattern string, originalHandler, wrappingHandler http.Handler) {
	m.router.handle(method, pattern, originalHandler, wrappingHandler)
}

// ServeHTTP implements a http.Handler
//
// It routes incoming http requests based on method and path extracting path parameters as it goes.
func (m Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.router.serveHTTP(w, r)
}

// Resolve resolves a URL path to its corresponding [Route] and hence http handler.
// If no corresponding route/handler is found, a zero value [Route] is returned.
//
// It is not intended for use in production settings, it is more of a dev/debugging tool.
// It is inspired by django's [resolve] url utility.
//
// [resolve]: https://docs.djangoproject.com/en/4.2/ref/urlresolvers/#django.urls.resolve
func (m Mux) Resolve(path string) Route {
	zero := Route{}

	u, err := url.Parse(path)
	if err != nil {
		return zero
	}

	{
		// todo: unify this logic with that found in `router.serveHTTP`
		segs := pathSegments(u.Path)
		for _, rt := range m.router.routes {
			if _, ok := rt.match(context.Background(), segs); ok {
				return rt
			}
		}
	}

	return zero
}

// Param gets the path/url parameter from the specified Context.
// It returns an empty string if the parameter was not found.
func Param(ctx context.Context, param string) string {
	vStr, ok := ctx.Value(muxContextKey(param)).(string)
	if !ok {
		return ""
	}
	return vStr
}
