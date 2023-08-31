# ong

[![Go Reference](https://pkg.go.dev/badge/github.com/komuw/ong.svg)](https://pkg.go.dev/github.com/komuw/ong)     
[![ci](https://github.com/komuw/ong/workflows/ong%20ci/badge.svg)](https://github.com/komuw/ong/actions)     
[![codecov](https://codecov.io/gh/komuw/ong/branch/main/graph/badge.svg?token=KMX47WCNK0)](https://codecov.io/gh/komuw/ong)     


Ong is a small http toolkit. 

It's name is derived from Tanzanian artiste, [Remmy Ongala](https://en.wikipedia.org/wiki/Remmy_Ongala).


Inspired by; `How I Write HTTP Web Services after Eight Years`[1][2] by Mat Ryer.    


You really should **not** use this library/toolkit.    
Instead, use the Go `net/http` package; and if you need some extra bits, may I suggest the awesome [github.com/gorilla](https://github.com/gorilla) web toolkit.    


This library is made just for me, it might be unsafe & it does not generally accept code contributions.       


```go
package main

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

func main() {
	l := log.New(context.Background(), os.Stdout, 1000)
	secretKey := "super-h@rd-Pa$1word"
	opts := config.WithOpts(
		"localhost",
		65081,
		secretKey,
		middleware.DirectIpStrategy,
		l,
	) // dev options.
	// alternatively for production:
	//   opts := config.LetsEncryptOpts("example.com", "secretKey", middleware.DirectIpStrategy, l, "hey@example.com")

	mux := mux.New(
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

	err := server.Run(mux, opts)
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

A more complete example can be found in the `example/` folder.      



### references:
1. https://www.youtube.com/watch?v=rWBSMsLG8po     
2. https://pace.dev/blog/2018/05/09/how-I-write-http-services-after-eight-years.html     
