# goweb

[![ci](https://github.com/komuw/goweb/workflows/goweb%20ci/badge.svg)](https://github.com/komuw/goweb/actions)
[![codecov](https://codecov.io/gh/komuw/goweb/branch/main/graph/badge.svg)](https://codecov.io/gh/komuw/goweb)


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

	"github.com/komuw/goweb/log"
	"github.com/komuw/goweb/middleware"
	"github.com/komuw/goweb/server"
)

func main() {
	api := myAPI{db:"someDb", l: someLogger}
	mux := server.NewMux(server.Routes{
		server.NewRoute(
			"check/",
			server.MethodGet,
			api.check(200),
			middleware.WithOpts("localhost"),
		),
	})

	err := server.Run(mux, server.DefaultOpts())
	if err != nil {
		mux.GetLogger().Error(err, log.F{
			"msg": "server.Run error",
		})
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


1. https://www.youtube.com/watch?v=rWBSMsLG8po     
2. https://pace.dev/blog/2018/05/09/how-I-write-http-services-after-eight-years.html     

