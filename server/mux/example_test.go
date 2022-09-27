package mux_test

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/komuw/ong/log"
	"github.com/komuw/ong/middleware"
	"github.com/komuw/ong/server/mux"
)

func LoginHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "welcome to your favorite website.")
	}
}

func BooksByAuthorHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		author := mux.Param(r.Context(), "author")
		_, _ = fmt.Fprint(w, fmt.Sprintf("fetching books by author: %s", author))
	}
}

func ExampleMux() {
	l := log.New(context.Background(), os.Stdout, 1000)
	mux := mux.NewMux(
		l,
		middleware.WithOpts("localhost", 8080, "secretKey", l),
		mux.Routes{
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
		},
	)

	server := &http.Server{
		Handler: mux,
		Addr:    ":8080",
	}
	err := server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
