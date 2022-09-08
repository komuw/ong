package main

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/komuw/ong/log"
	"github.com/komuw/ong/middleware"
	"github.com/komuw/ong/server"
)

// Taken mainly from the talk; "How I Write HTTP Web Services after Eight Years" by Mat Ryer
// 1. https://www.youtube.com/watch?v=rWBSMsLG8po
// 2. https://pace.dev/blog/2018/05/09/how-I-write-http-services-after-eight-years.html

func main() {
	api := NewMyApi("someDb")
	l := log.New(context.Background(), os.Stdout, 1000)
	secretKey := []byte("key should be 32bytes and random")
	mux := server.NewMux(
		l,
		middleware.WithOpts("localhost", 65081, secretKey, l),
		server.Routes{
			server.NewRoute(
				"/api",
				server.MethodPost,
				api.handleAPI(),
			),
			server.NewRoute(
				"serveDirectory",
				server.MethodAll,
				middleware.BasicAuth(api.handleFileServer(), "user", "some-long-passwd"),
			),
			server.NewRoute(
				"check/",
				server.MethodAll,
				api.check(200),
			),
			server.NewRoute(
				"login",
				server.MethodAll,
				api.login(),
			),
		})

	_, _ = server.CreateDevCertKey()
	err := server.Run(mux, server.DevOpts(), l)
	if err != nil {
		l.Error(err, log.F{"msg": "server.Run error"})
		os.Exit(1)
	}
}

// myAPI represents component as a struct, shared dependencies as fields, no global state
type myAPI struct {
	db string
	l  log.Logger
}

func NewMyApi(db string) myAPI {
	return myAPI{
		db: db,
		l:  log.New(context.Background(), os.Stdout, 1000),
	}
}

func (m myAPI) handleFileServer() http.HandlerFunc {
	// Do NOT let `http.FileServer` be able to serve your root directory.
	// Otherwise, your .git folder and other sensitive info(including http://localhost:65080/main.go) may be available
	// instead create a folder that only has your templates and server that.
	fs := http.FileServer(http.Dir("./stuff"))
	realHandler := http.StripPrefix("somePrefix", fs).ServeHTTP
	return func(w http.ResponseWriter, req *http.Request) {
		m.l.Info(log.F{"msg": "handleFileServer", "redactedURL": req.URL.Redacted()})
		realHandler(w, req)
	}
}

// Handlers are methods on the server which gives them access to dependencies
// Remember, other handlers have access to `s` too, so be careful with data races
// Why return `http.HandlerFunc` instead of `http.Handler`?
// `HandlerFunc` implements `Handler` interface so they are kind of interchangeable
// Pick whichever is easier for you to use. Sometimes you might have to convert between them
func (m myAPI) handleAPI() http.HandlerFunc {
	// allows for handler specific setup
	thing := func() int {
		return 42
	}
	var once sync.Once
	var serverStart time.Time

	return func(w http.ResponseWriter, r *http.Request) {
		// intialize somethings only once for perf
		once.Do(func() {
			m.l.Info(log.F{"msg": "called only once during the first request"})
			serverStart = time.Now()
		})

		// use thing
		ting := thing()
		if ting != 42 {
			http.Error(w, "thing ought to be 42", http.StatusBadRequest)
			return
		}

		res := fmt.Sprintf("serverStart=%v\n. Hello. answer to life is %v \n", serverStart, ting)
		_, _ = io.WriteString(w, res)
	}
}

// you can take arguments for handler specific dependencies
func (m myAPI) check(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cspNonce := middleware.GetCspNonce(r.Context())
		csrfToken := middleware.GetCsrfToken(r.Context())
		m.l.Info(log.F{"msg": "check called", "cspNonce": cspNonce, "csrfToken": csrfToken})

		_, _ = fmt.Fprint(w, "hello from check/ endpoint")
		// use code, which is a dependency specific to this handler
		w.WriteHeader(code)
	}
}

func (m myAPI) login() http.HandlerFunc {
	tmpl, err := template.New("myTpl").Parse(`<!DOCTYPE html>
<html>

<body>
    <script nonce="{{.CspNonceValue}}">
	    console.log("hello world");
	</script>

	<h2>Welcome to awesome website.</h2>
	<form method="POST">
	<label>Email:</label><br>
	<input type="text" id="email" name="email"><br>
	<label>First Name:</label><br>
	<input type="text" id="firstName" name="firstName"><br>

	<input type="hidden" id="{{.CsrfTokenName}}" name="{{.CsrfTokenName}}" value="{{.CsrfTokenValue}}"><br>
	<input type="submit">
	</form>

</body>
</html>`)
	if err != nil {
		panic(err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			data := struct {
				CsrfTokenName  string
				CsrfTokenValue string
				CspNonceValue  string
			}{
				CsrfTokenName:  middleware.CsrfTokenFormName,
				CsrfTokenValue: middleware.GetCsrfToken(r.Context()),
				CspNonceValue:  middleware.GetCspNonce(r.Context()),
			}

			if err = tmpl.Execute(w, data); err != nil {
				panic(err)
			}
			return
		}

		if err = r.ParseForm(); err != nil {
			panic(err)
		}

		_, _ = fmt.Fprintf(w, "you have submitted: %s", r.Form)
	}
}
