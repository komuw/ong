package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/komuw/goweb/log"
	"github.com/komuw/goweb/middleware"
	"github.com/komuw/goweb/server"
)

// Taken mainly from the talk; "How I Write HTTP Web Services after Eight Years" by Mat Ryer
// 1. https://www.youtube.com/watch?v=rWBSMsLG8po
// 2. https://pace.dev/blog/2018/05/09/how-I-write-http-services-after-eight-years.html

func main() {
	api := NewMyApi("someDb")

	mux := server.NewMux(server.Routes{
		server.NewRoute("/api", server.MethodPost, api.handleAPI(), middleware.WithOpts("localhost")),
		server.NewRoute("serveDirectory", server.MethodAll, middleware.BasicAuth(api.handleFileServer(), "user", "passwd"), middleware.WithOpts("localhost")),
		server.NewRoute("check/", server.MethodGet, api.check(200), middleware.WithOpts("localhost")),
	})

	err := server.Run(mux, server.DefaultOpts())
	if err != nil {
		mux.GetLogger().Error(err, log.F{
			"msg": "server.Run error",
		})
		os.Exit(1)
	}
}

// myAPI represents component as struct shared dependencies as fields
// no global state
type myAPI struct {
	db string
	l  log.Logger
}

func NewMyApi(db string) myAPI {
	return myAPI{
		db: "someDb",
		l:  log.New(context.Background(), os.Stdout, 1000, false),
	}
}

func (s myAPI) handleFileServer() http.HandlerFunc {
	// Do NOT let `http.FileServer` be able to serve your root directory.
	// Otherwise, your .git folder and other sensitive info(including http://localhost:8080/main.go) may be available
	// instead create a folder that only has your templates and server that.
	fs := http.FileServer(http.Dir("./stuff"))
	realHandler := http.StripPrefix("somePrefix", fs).ServeHTTP
	return func(w http.ResponseWriter, req *http.Request) {
		s.l.Info(log.F{"msg": "handleFileServer", "redactedURL": req.URL.Redacted()})
		realHandler(w, req)
	}
}

// Handlers are methods on the server which gives them access to dependencies
// Remember, other handlers have access to `s` too, so be careful with data races
// Why return `http.HandlerFunc` instead of `http.Handler`?
// `HandlerFunc` implements `Handler` interface so they are kind of interchangeable
// Pick whichever is easier for you to use. Sometimes you might have to convert between them
func (s myAPI) handleAPI() http.HandlerFunc {
	// allows for handler specific setup
	thing := func() int {
		return 42
	}
	var once sync.Once
	var serverStart time.Time

	return func(w http.ResponseWriter, r *http.Request) {
		// intialize somethings only once for perf
		once.Do(func() {
			s.l.Info(log.F{"msg": "called only once during the first request"})
			serverStart = time.Now()
		})

		// use thing
		ting := thing()
		if ting != 42 {
			http.Error(w, "thing ought to be 42", http.StatusBadRequest)
			return
		}

		res := fmt.Sprintf("serverStart=%v\n. Hello. answer to life is %v \n", serverStart, ting)
		_, _ = w.Write([]byte(res))
	}
}

// you can take arguments for handler specific dependencies
func (s myAPI) check(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cspNonce := middleware.GetCspNonce(r.Context())
		csrfToken := middleware.GetCsrfToken(r.Context())
		s.l.Info(log.F{"msg": "check called", "cspNonce": cspNonce, "csrfToken": csrfToken})

		// use code, which is a dependency specific to this handler
		w.WriteHeader(code)
	}
}
