package main

import (
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// myAPI rep component as struct
// shared dependencies as fields
// no global state
type myAPI struct {
	db     string
	router *http.ServeMux // some router
	logger *log.Logger    // some logger, maybe
}

// Make `myAPI` implement the http.Handler interface(https://golang.org/pkg/net/http/#Handler)
// use myAPI wherever you could use http.Handler(eg ListenAndServe)
func (s *myAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *myAPI) GetLogger() *log.Logger {
	if s.logger == nil {
		s.logger = log.New(os.Stdout, "logger: ", log.Lshortfile)
	}
	return s.logger
}

// Have one place for all routes.
// You can even move it to a routes.go file
func (s *myAPI) Routes() {
	s.router.HandleFunc("/api/",
		s.flocOptOut(s.handleAPI()),
	)
	s.router.HandleFunc("/greeting",
		// you can even have your handler take a `*template.Template` dependency
		s.flocOptOut(
			s.Auth(s.handleGreeting(202)),
		),
	)
	s.router.HandleFunc("/serveDirectory",
		s.flocOptOut(
			s.Auth(s.handleFileServer()),
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
		s.GetLogger().Println(req.URL.Redacted())
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
			s.GetLogger().Println("called only once during the first request")
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
		w.WriteHeader(code)
	}
}

// TODO: add these security headers;
// https://web.dev/security-headers/

// flocOptOut disables floc which is otherwise ON by default
// see: https://github.com/WICG/floc#opting-out-of-computation
func (s *myAPI) flocOptOut(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// code that is ran b4 wrapped handler
		w.Header().Set("Permissions-Policy", "interest-cohort=()")
		wrappedHandler(w, r)
	}
}

// middleware are just go functions
// you can run code before and/or after the wrapped hanlder
func (s *myAPI) Auth(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	const realm = "enter username and password"
	return func(w http.ResponseWriter, r *http.Request) {
		// code that is ran b4 wrapped handler
		s.GetLogger().Println("code ran BEFORE wrapped handler")
		username, _, _ := r.BasicAuth()

		if username == "" { //|| pass == ""
			w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if subtle.ConstantTimeCompare([]byte(username), []byte("admin")) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		wrappedHandler(w, r)
		// you can also run code after wrapped handler here
		// you can even choose not to call wrapped handler at all
		s.GetLogger().Println("code ran AFTER wrapped handler")
	}
}
