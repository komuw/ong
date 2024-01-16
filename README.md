# ong

[![Go Reference](https://pkg.go.dev/badge/github.com/komuw/ong.svg)](https://pkg.go.dev/github.com/komuw/ong)     
[![ci](https://github.com/komuw/ong/actions/workflows/ci.yml/badge.svg)](https://github.com/komuw/ong/actions)     
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
	secretKey := "super-h@rd-Pas1word"
	opts := config.WithOpts(
		"localhost",
		65081,
		secretKey,
		config.DirectIpStrategy,
		l,
	) // dev options.
	// alternatively for production:
	//   opts := config.LetsEncryptOpts("example.com", "secretKey", config.DirectIpStrategy, l, "hey@example.com", []string{"api.example.com", "example.com"})

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
```

`go run -race ./...`       

A more complete example can be found in the `example/` folder.      



### References:
1. https://www.youtube.com/watch?v=rWBSMsLG8po     
2. https://pace.dev/blog/2018/05/09/how-I-write-http-services-after-eight-years.html     


### Features:
The simplest production ready program using `ong` http toolkit would be something like;
```go
package main

import (
    "fmt"
    "net/http"
    "os"

    "github.com/komuw/ong/config"
	"github.com/komuw/ong/log"
    "github.com/komuw/ong/mux"
    "github.com/komuw/ong/server"
)

func main() {
    logger := log.New(context.Background(), os.Stdout, 1000)
    domain := "example.com"
    secretKey := "super-h@rd-Pas1word"
    email := "hey@example.com"
    opts := config.LetsEncryptOpts(domain, secretKey, config.DirectIpStrategy, logger, email, []string{domain})

    mx := mux.New(opts, nil, mux.NewRoute("hello/", mux.MethodGet, hello()))
    _ = server.Run(mx, opts)
}

func hello() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprint(w, "hello")
    }
}
```
If you do that, these are the features you would enjoy automatically without doing any extra configuration;
1. Http server. You get a server that automatically;
   - sets GOMEMLIMIT & GOMAXPROCS to match linux container memory & cpu quotas.  
   - fetches and auto renews TLS certificates from [letsencrypt](https://letsencrypt.org/) or any other compatible ACME authority.
   - serves pprof endpoints that are secured by basic authentication. The `secretKey` is the username and password.
   - handles automatic http->https redirection.
   - implements robust http timeouts to prevent attacks.
   - limits size of request bodies to prevent attacks.
   - shutsdown cleanly after receiving termination signals. If running in kubernetes, the shutdown is [well co-ordinated](https://twitter.com/thockin/status/1560398974929973248) to prevent errors.
2. Automatic ratelimiting.
3. Automatic loadshedding.
4. Automatic proper handling for [CORS](https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS)
5. Automatic [CSRF](https://developer.mozilla.org/en-US/docs/Glossary/CSRF) protection.
6. Automatic logging of erroring requests with correlation IDs included.
   The logging is lightweight so it only logs when an error occurs. Importantly, when the error occurs, it also includes all the log statements including the non-error ones.
7. Automatic recovery of panics in http handlers and logging of the same including stack traces.
8. Automatic addition of the [real client IP](https://adam-p.ca/blog/2022/03/x-forwarded-for/) to request context.
9. Protection against inadvertent form re-submission.
10. Automatically sets appropriate secure headers(`X-Content-Type-Options`, `Content-Security-Policy`, `X-Frame-Options`, `Cross-Origin-Resource-Policy`, `Cross-Origin-Opener-Policy`, `Referrer-Policy`, `Strict-Transport-Security`)
11. Automatic addition of TLS fingerprint to request context. 
12. Set's up secure authenticated encrypted http sessions.
13. Uses a http request multiplexer that; 
   - panics(during application startup) if there are any conflicting routes.
   - has a debugging tool where if given a url, it will return the corresponding http handler for that url.
   - can capture path parameters

Those are the automatic ones. There are a few additional features that you can opt into;
1. A [http client](https://pkg.go.dev/github.com/komuw/ong/client) that properly handles [server-side request forgery](https://en.wikipedia.org/wiki/Server-side_request_forgery) attacks. 
2. A [cookie](https://pkg.go.dev/github.com/komuw/ong/cookie) package that enables you to work with both plain text cookies and also authenticated encrypted cookies.
3. A [cryptography](https://pkg.go.dev/github.com/komuw/ong/cry) package that simplifies using authenticated encryption and also hashing.
4. An [errors](https://pkg.go.dev/github.com/komuw/ong/errors) package that includes error wrapping and stack trace support.
5. An [id](https://pkg.go.dev/github.com/komuw/ong/id) package that can generate unique random human friendly identifiers, as well as uuid4(does not leak its creation time) and uuid8(has good database locality).
6. A [log](https://pkg.go.dev/github.com/komuw/ong/log) package that implements [slog.Logger](https://pkg.go.dev/log/slog#Logger) and is backed by an [slog.Handler](https://pkg.go.dev/log/slog#Handler) that stores log messages into a circular buffer.  
7. A [sess](https://pkg.go.dev/github.com/komuw/ong/sess) package that makes it easy to work with http sessions that are backed by tamper-proof & encrypted cookies.   
8. A [sync](https://pkg.go.dev/github.com/komuw/ong/sync) package that makes it easier to work with groups of goroutines working on subtasks of a common task.


