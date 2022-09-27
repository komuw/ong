package server_test

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/komuw/ong/log"
	"github.com/komuw/ong/middleware"
	"github.com/komuw/ong/mux"
	"github.com/komuw/ong/server"
)

func main() {
	l := log.New(context.Background(), os.Stdout, 1000)
	secretKey := "hard-password"
	mux := mux.New(
		l,
		middleware.WithOpts("localhost", 65081, secretKey, l),
		mux.Routes{
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
		})

	opts := server.DevOpts() // dev options.
	// alternatively for production:
	//   opts := server.LetsEncryptOpts("email@email.com", "*.some-domain.com")
	err := server.Run(mux, opts, l)
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
		_, _ = fmt.Fprint(w, fmt.Sprintf("Age is %s", age))
	}
}
