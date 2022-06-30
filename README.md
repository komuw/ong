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
	api := myAPI{db:"someDb", l: someLogger}
	mux := server.NewMux(
		server.Routes{
		    server.NewRoute(
			    "check/",
			    server.MethodGet,
			    api.check(200),
			    middleware.WithOpts("localhost"),
		   ),
	    })

	err := server.Run(mux, server.DefaultOpts())
	if err != nil {
		mux.GetLogger().Error(err, log.F{"msg": "server.Run error"})
		os.Exit(1)
	}
}

type myAPI struct {
	db string
	l  log.Logger
}

func (s myAPI) check(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cspNonce := middleware.GetCspNonce(r.Context())
		csrfToken := middleware.GetCsrfToken(r.Context())
		s.l.Info(log.F{"msg": "check called", "cspNonce": cspNonce, "csrfToken": csrfToken})

		// use code, which is a dependency specific to this handler
		w.WriteHeader(code)
	}
}
```

`go run -race ./...`     

To use tls:
```go
_, _ = server.CreateDevCertKey()
err := server.Run(mux, server.DefaultTlsOpts())
```


1. https://www.youtube.com/watch?v=rWBSMsLG8po     
2. https://pace.dev/blog/2018/05/09/how-I-write-http-services-after-eight-years.html     
