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
)

// myAPI rep component as struct
// shared dependencies as fields
// no global state
type myAPI struct {
	db     string
	router *http.ServeMux // some router
	logger log.Logger     // some logger, maybe
}

// Make `myAPI` implement the http.Handler interface(https://golang.org/pkg/net/http/#Handler)
// use myAPI wherever you could use http.Handler(eg ListenAndServe)
func (s *myAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *myAPI) GetLogger() log.Logger {
	return log.New(context.Background(), os.Stdout, 1000, false)
}

// Have one place for all routes.
// You can even move it to a routes.go file
func (s *myAPI) Routes() {
	s.router.HandleFunc("/api/",
		middleware.All(s.handleAPI(), middleware.WithOpts("localhost")),
	)
	s.router.HandleFunc("/greeting",
		// you can even have your handler take a `*template.Template` dependency
		middleware.All(
			middleware.BasicAuth(s.handleGreeting(202), "user", "passwd"),
			middleware.WithOpts("localhost"),
		),
	)
	s.router.HandleFunc("/serveDirectory",
		middleware.All(
			middleware.BasicAuth(s.handleFileServer(), "user", "passwd"),
			middleware.WithOpts("localhost"),
		),
	)

	s.router.HandleFunc("/check",
		middleware.All(s.handleGreeting(200),
			middleware.WithOpts("localhost"),
		),
	)

	// etc
}

func (s *myAPI) handleFileServer() http.HandlerFunc {
	// Do NOT let `http.FileServer` be able to serve your root directory.
	// Otherwise, your .git folder and other sensitive info(including http://localhost:8080/main.go) may be available
	// instead create a folder that only has your templates and server that.
	fs := http.FileServer(http.Dir("./stuff"))
	realHandler := http.StripPrefix("somePrefix", fs).ServeHTTP
	return func(w http.ResponseWriter, req *http.Request) {
		s.GetLogger().Info(log.F{"msg": "handleFileServer", "redactedURL": req.URL.Redacted()})
		realHandler(w, req)
	}
}

// Handlers are methods on the server which gives them access to dependencies
// Remember, other handlers have access to `s` too, so be careful with data races
// Why return `http.HandlerFunc` instead of `http.Handler`?
// `HandlerFunc` implements `Handler` interface so they are kind of interchangeable
// Pick whichever is easier for you to use. Sometimes you might have to convert between them
func (s *myAPI) handleAPI() http.HandlerFunc {
	// allows for handler specific setup
	thing := func() int {
		return 42
	}
	var once sync.Once
	var serverStart time.Time

	// return the handler
	return func(w http.ResponseWriter, r *http.Request) {
		// intialize somethings only once for perf
		once.Do(func() {
			s.GetLogger().Info(log.F{"msg": "called only once during the first request"})

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
func (s *myAPI) handleGreeting(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// use code, which is a dependency specific to this handler

		nonce := middleware.GetCspNonce(r.Context())
		s.GetLogger().Info(log.F{"msg": "handleGreeting", "nonce": nonce})

		w.WriteHeader(code)
	}
}
