package main

// Taken mainly from the talk; How I Write HTTP Web Services after Eight Years
// - https://www.youtube.com/watch?v=rWBSMsLG8po -  Mat Ryer.

import (
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
func (s myAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// Have one place for all routes.
// You can even move it to a routes.go file
func (s myAPI) routes() {
	s.router.HandleFunc("/api/", s.handleAPI())
	s.router.HandleFunc("/greeting", s.Auth(
		s.handleGreeting(202))) // you can even have your handler take a `*template.Template` dependency
	// etc
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

	// return the handler
	return func(w http.ResponseWriter, r *http.Request) {
		// intialize somethings only once for perf
		once.Do(func() {
			s.logger.Println("called only once during the first request")
			serverStart = time.Now()
		})

		// use thing
		ting := thing()
		if ting != 42 {
			http.Error(w, "thing ought to be 42", http.StatusBadRequest)
			return
		}

		res := fmt.Sprintf("serverStart=%v\n. Hello. answer to life is %v \n", serverStart, ting)
		w.Write([]byte(res))
	}
}

// you can take arguments for handler specific dependencies
func (s myAPI) handleGreeting(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// use code, which is a dependency specific to this handler
		w.WriteHeader(code)
	}
}

// middleware are just go functions
// you can run code before and/or after the wrapped hanlder
func (s myAPI) Auth(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// code that is ran b4 wrapped handler
		fmt.Println("code ran BEFORE wrapped handler")
		username, _, _ := r.BasicAuth()
		if username != "admin" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		wrappedHandler(w, r)
		// you can also run code after wrapped handler here
		// you can even choose not to call wrapped handler at all
		fmt.Println("code ran AFTER wrapped handler")
	}
}

func main() {
	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// TODO: does the server have to be a pointer?
	api := myAPI{
		db:     "someDb",
		router: http.NewServeMux(),
		logger: log.New(os.Stdout, "logger: ", log.Lshortfile),
	}
	api.routes()

	serverPort := ":8080"
	server := &http.Server{
		Addr:    serverPort,
		Handler: api,
		// 1. https://blog.simon-frey.eu/go-as-in-golang-standard-net-http-config-will-break-your-production
		// 2. https://blog.cloudflare.com/exposing-go-on-the-internet/
		// 3. https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
		ReadHeaderTimeout: 1 * time.Second,
		ReadTimeout:       1 * time.Second,
		WriteTimeout:      1 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	api.logger.Printf("server listening at port %s", serverPort)
	err := server.ListenAndServe()
	return err
}
