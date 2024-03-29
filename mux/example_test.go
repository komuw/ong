package mux_test

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/komuw/ong/config"
	"github.com/komuw/ong/log"
	"github.com/komuw/ong/mux"
)

func LoginHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "welcome to your favorite website.")
	}
}

func BooksByAuthorHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		author := mux.Param(r.Context(), "author")
		_, _ = fmt.Fprintf(w, "fetching books by author: %s", author)
	}
}

func ExampleMuxer() {
	l := log.New(context.Background(), os.Stdout, 1000)
	mx := mux.New(
		config.WithOpts("localhost", 8080, "super-h@rd-Pas1word", config.DirectIpStrategy, l),
		nil,
		mux.NewRoute(
			"login/",
			mux.MethodGet,
			LoginHandler(),
		),
		mux.NewRoute(
			"/books/:author",
			mux.MethodAll,
			BooksByAuthorHandler(),
		),
	)

	server := &http.Server{
		Handler: mx,
		Addr:    ":8080",
	}
	err := server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}

func ExampleMuxer_Resolve() {
	l := log.New(context.Background(), os.Stdout, 1000)
	mx := mux.New(
		config.WithOpts("localhost", 8080, "super-h@rd-Pas1word", config.DirectIpStrategy, l),
		nil,
		mux.NewRoute(
			"login/",
			mux.MethodGet,
			LoginHandler(),
		),
		mux.NewRoute(
			"/books/:author",
			mux.MethodAll,
			BooksByAuthorHandler(),
		),
	)

	fmt.Println(mx.Resolve("nonExistentPath"))
	fmt.Println(mx.Resolve("login/"))
	fmt.Println(mx.Resolve("https://localhost/books/SidneySheldon"))
}
