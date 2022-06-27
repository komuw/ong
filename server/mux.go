package server

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/komuw/goweb/log"
	"github.com/komuw/goweb/middleware"
)

type MuxOpts struct {
	pattern string
	handler http.HandlerFunc
	opts    middleware.Opts
}

func NewMuxOpts(pattern string,
	handler http.HandlerFunc,
	opts middleware.Opts,
) MuxOpts {
	return MuxOpts{
		pattern: pattern,
		handler: handler,
		opts:    opts,
	}
}

type mux struct {
	router *http.ServeMux // some router
}

func NewMux(mo []MuxOpts) *mux {
	m := &mux{
		router: http.NewServeMux(),
	}
	for _, v := range mo {
		m.addPattern(v.pattern,
			middleware.All(v.handler, v.opts),
		)
	}
	return m
}

func (m *mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.router.ServeHTTP(w, r)
}

func (m *mux) GetLogger() log.Logger {
	return log.New(context.Background(), os.Stdout, 1000, false)
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
