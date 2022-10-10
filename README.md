# ong

[![Go Reference](https://pkg.go.dev/badge/github.com/komuw/ong.svg)](https://pkg.go.dev/github.com/komuw/ong)     
[![ci](https://github.com/komuw/ong/workflows/ong%20ci/badge.svg)](https://github.com/komuw/ong/actions)     
[![codecov](https://codecov.io/gh/komuw/ong/branch/main/graph/badge.svg)](https://codecov.io/gh/komuw/ong)     


Ong is a small http toolkit. 

It's name is derived from Tanzanian artiste, [Remmy Ongala](https://en.wikipedia.org/wiki/Remmy_Ongala).


Taken mainly from the talk; `How I Write HTTP Web Services after Eight Years`[1][2] by Mat Ryer.    


You really should not be using this code/library. The Go `net/http` package is more than enough.    
If you need some extra bits, may I suggest the awesome [github.com/gorilla](https://github.com/gorilla) web toolkit.    


This library is made just for me, it might be unsafe & it does not generally accept code contributions.       


```go
package main

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
	l := log.New(os.Stdout, 1000)
	secretKey := "hard-password"
	mux := mux.New(
		l,
		middleware.WithOpts("localhost", 65081, secretKey, l),
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
		_, _ = fmt.Fprintf(w, "Age is %s", age)
	}
}
```

`go run -race ./...`     


1. https://www.youtube.com/watch?v=rWBSMsLG8po     
2. https://pace.dev/blog/2018/05/09/how-I-write-http-services-after-eight-years.html     
