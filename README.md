# ong

[![ci](https://github.com/komuw/ong/workflows/ong%20ci/badge.svg)](https://github.com/komuw/ong/actions)
[![codecov](https://codecov.io/gh/komuw/ong/branch/main/graph/badge.svg)](https://codecov.io/gh/komuw/ong)


Ong is a small web toolkit. 

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
	"sync"
	"time"

	"github.com/komuw/ong/log"
	"github.com/komuw/ong/middleware"
	"github.com/komuw/ong/server"
)

func main() {
	l := log.New(context.Background(), os.Stdout, 1000)
	mux := server.NewMux(
		l,
		middleware.WithOpts("localhost", 8081, l),
		server.Routes{
		    server.NewRoute(
			    "hello/",
			    server.MethodGet,
			    hello("hello world"),
		   ),
	    })

    _, _ = server.CreateDevCertKey()
	err := server.Run(mux, server.DevOpts())
	if err != nil {
		fmt.Prinln(err)
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
```

`go run -race ./...`     


To use tls with certificates from letsencrypt:
```go
email := "admin@example.com"
domain := "*.example.com"
err := server.Run(mux, server.LetsEncryptOpts(email, domain))
```


1. https://www.youtube.com/watch?v=rWBSMsLG8po     
2. https://pace.dev/blog/2018/05/09/how-I-write-http-services-after-eight-years.html     
