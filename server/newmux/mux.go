// TODO: docs.
package mux

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"runtime"
	"strings"
)

// Most of the code here is insipired by(or taken from):
//   (a) https://github.com/matryer/way whose license(MIT) can be found here: https://github.com/matryer/way/blob/9632d0c407b008073d19d0c4da1e0fc3e9477508/LICENSE

// wayContextKey is the context key type for storing
// parameters in context.Context.
type wayContextKey string

type route struct {
	method  string
	segs    []string
	handler http.Handler
}

func (r route) String() string {
	return fmt.Sprintf("route{method: %s, segs: %s}", r.method, r.segs)
}

func (r route) match(ctx context.Context, router *Router, segs []string) (context.Context, bool) {
	if len(segs) > len(r.segs) {
		return nil, false
	}
	for i, seg := range r.segs {
		if i > len(segs)-1 {
			return nil, false
		}
		isParam := false
		if strings.HasPrefix(seg, ":") {
			isParam = true
			seg = strings.TrimPrefix(seg, ":")
		}
		if !isParam { // verbatim check
			if seg != segs[i] {
				return nil, false
			}
		}
		if isParam {
			ctx = context.WithValue(ctx, wayContextKey(seg), segs[i])
		}
	}
	return ctx, true
}

// Router routes HTTP requests.
type Router struct {
	routes []route
	// NotFound is the http.Handler to call when no routes
	// match. By default uses http.NotFoundHandler().
	NotFound http.Handler
}

// NewRouter makes a new Router.
func NewRouter() *Router {
	return &Router{
		NotFound: http.NotFoundHandler(),
	}
}

func (r *Router) pathSegments(p string) []string {
	return strings.Split(strings.Trim(p, "/"), "/")
}

// Handle adds a handler with the specified method and pattern.
// Method can be any HTTP method string or "*" to match all methods.
// Pattern can contain path segments such as: /item/:id which is
// accessible via the Param function.
func (r *Router) Handle(method, pattern string, handler http.Handler) {
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

	// Try and detect conflict before adding a new route.
	r.detectConflict(method, pattern, handler)

	route := route{
		method:  strings.ToLower(method),
		segs:    r.pathSegments(pattern),
		handler: handler,
	}
	r.routes = append(r.routes, route)
}

// HandleFunc is the http.HandlerFunc alternative to http.Handle.
func (r *Router) HandleFunc(method, pattern string, fn http.HandlerFunc) {
	r.Handle(method, pattern, fn)
}

// ServeHTTP routes the incoming http.Request based on method and path
// extracting path parameters as it goes.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	method := strings.ToLower(req.Method)
	segs := r.pathSegments(req.URL.Path)
	for _, route := range r.routes {
		if route.method != method && route.method != "*" {
			// TODO: fix how we handle "*" methods.
			continue
		}
		if ctx, ok := route.match(req.Context(), r, segs); ok {
			route.handler.ServeHTTP(w, req.WithContext(ctx))
			return
		}
	}
	r.NotFound.ServeHTTP(w, req)
}

// Param gets the path parameter from the specified Context.
// Returns an empty string if the parameter was not found.
func Param(ctx context.Context, param string) string {
	vStr, ok := ctx.Value(wayContextKey(param)).(string)
	if !ok {
		return ""
	}
	return vStr
}

func (r *Router) detectConflict(method, pattern string, handler http.Handler) {
	// Conflicting routes are a bad thing.
	// They can be a source of bugs and confusion.
	// see: https://www.alexedwards.net/blog/which-go-router-should-i-use

	incomingSegments := r.pathSegments(pattern)
	for _, route := range r.routes {
		existingSegments := route.segs
		sameLen := len(incomingSegments) == len(existingSegments)
		if !sameLen {
			// no conflict
			break
		}

		panicMsg := fmt.Sprintf(`
You are trying to add
  pattern: %s
  method: %s
  handler: %v
However
  pattern: %s
  method: %s
  handler: %v
already exists and would conflict.`,
			pattern,
			method,
			runtime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name(),
			strings.Join(route.segs, "/"),
			strings.ToUpper(route.method),
			runtime.FuncForPC(reflect.ValueOf(route.handler).Pointer()).Name(),
		)

		for _, v := range incomingSegments {
			if strings.Contains(v, ":") {
				panic(panicMsg)
			}
		}
		for _, v := range existingSegments {
			if strings.Contains(v, ":") {
				panic(panicMsg)
			}
		}
	}
}
