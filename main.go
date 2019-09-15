package main

// Taken mainly from the talk; How I Write HTTP Web Services after Eight Years
// - https://www.youtube.com/watch?v=rWBSMsLG8po -  Mat Ryer.

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// customServer rep component as struct
// shared dependencies as fields
// no global state
type customServer struct {
	db     string
	router *http.ServeMux // some router
	logger *log.Logger    // some logger, maybe
}

// Make `customServer` implement the http.Handler interface(https://golang.org/pkg/net/http/#Handler)
// use customServer wherever you could use http.Handler(eg ListenAndServe)
func (s *customServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// Have one place for all routes.
// You can even move it to a routes.go file
func (s *customServer) routes() {
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
func (s *customServer) handleAPI() http.HandlerFunc {
	// allows for handler specific setup
	thing := func() int {
		return 42
	}

	// return the handler
	return func(w http.ResponseWriter, r *http.Request) {
		// use thing
		if thing() != 42 {
			http.Error(w, "thing ought to be 42", http.StatusBadRequest)
			return
		}
	}
}

// you can take arguments for handler specific dependencies
func (s *customServer) handleGreeting(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// use code, which is a dependency specific to this handler
		w.WriteHeader(code)
	}
}

// middleware are just go functions
// you can run code before and/or after the wrapped hanlder
func (s *customServer) Auth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, _, _ := r.BasicAuth()
		if username != "admin" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		h(w, r)
		// you can also run code after wrapped handler here
		// you can even choose not to call wrapped handler at all
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
	srv := &customServer{
		db:     "someDb",
		router: http.NewServeMux(),
		logger: log.New(os.Stdout, "logger: ", log.Lshortfile),
	}
	srv.routes()

	serverPort := ":8080"
	server := &http.Server{
		Addr:         serverPort,
		Handler:      srv,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
		IdleTimeout:  120 * time.Second}

	srv.logger.Printf("server listening at port %s", serverPort)
	err := server.ListenAndServe()
	return err
}
