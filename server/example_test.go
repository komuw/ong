package server_test

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/komuw/ong/config"
	"github.com/komuw/ong/log"
	"github.com/komuw/ong/middleware"
	"github.com/komuw/ong/mux"
	"github.com/komuw/ong/server"
)

func ExampleRun() {
	l := log.New(context.Background(), os.Stdout, 1000)
	secretKey := "super-h@rd-Pas1word"
	opts := config.WithOpts(
		"localhost",
		65081,
		secretKey,
		config.DirectIpStrategy,
		l,
	) // dev options.
	// alternatively for production:
	//   opts := config.LetsEncryptOpts(...)

	mx := mux.New(
		opts,
		nil,
		mux.NewRoute(
			"hello/",
			mux.MethodGet,
			hello("hello world"),
		),
		mux.NewRoute(
			"check/:age/",
			mux.MethodAll,
			check(),
		),
	)

	err := server.Run(mx, opts)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func hello(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cspNonce := middleware.GetCspNonce(r.Context())
		csrfToken := middleware.GetCsrfToken(r.Context())
		fmt.Printf("hello called cspNonce: %s, csrfToken: %s", cspNonce, csrfToken)

		// use msg, which is a dependency specific to this handler
		fmt.Fprint(w, msg)
	}
}

func check() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		age := mux.Param(r.Context(), "age")
		_, _ = fmt.Fprintf(w, "Age is %s", age)
	}
}
