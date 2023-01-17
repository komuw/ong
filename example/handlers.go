package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/komuw/ong/cookie"
	"github.com/komuw/ong/log"
	"github.com/komuw/ong/middleware"
	"github.com/komuw/ong/mux"
	"github.com/komuw/ong/sess"
)

// myAPI represents component as a struct, shared dependencies as fields, no global state
type myAPI struct {
	db string
	l  log.Logger
}

func NewMyApi(db string) myAPI {
	return myAPI{
		db: db,
		l:  log.New(os.Stdout, 1000),
	}
}

func (m myAPI) handleFileServer() http.HandlerFunc {
	// Do NOT let `http.FileServer` be able to serve your root directory.
	// Otherwise, your .git folder and other sensitive info(including http://localhost:65080/main.go) may be available
	// instead create a folder that only has your templates and server that.
	fs := http.FileServer(http.Dir("./stuff"))
	realHandler := http.StripPrefix("somePrefix", fs).ServeHTTP
	return func(w http.ResponseWriter, r *http.Request) {
		reqL := m.l.WithCtx(r.Context())

		reqL.Info(log.F{"msg": "handleFileServer", "redactedURL": r.URL.Redacted()})
		realHandler(w, r)
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
		reqL := m.l.WithCtx(r.Context())

		// intialize somethings only once for perf
		once.Do(func() {
			reqL.Info(log.F{"msg": "called only once during the first request"})
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
func (m myAPI) check(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reqL := m.l.WithCtx(r.Context())

		cspNonce := middleware.GetCspNonce(r.Context())
		csrfToken := middleware.GetCsrfToken(r.Context())
		reqL.Info(log.F{"msg": "check called", "cspNonce": cspNonce, "csrfToken": csrfToken})

		cartID := "afakHda8eqL"
		sess.SetM(r, sess.M{
			"name":    "John Doe",
			"age":     "88",
			"cart_id": cartID,
		})

		reqL.WithImmediate().Info(log.F{"cart_id": sess.Get(r, "cart_id")})
		if sess.Get(r, "cart_id") != "" {
			if sess.Get(r, "cart_id") != cartID {
				panic("wrong cartID")
			}
		}

		age := mux.Param(r.Context(), "age")
		// use msg, which is a dependency specific to this handler
		_, _ = fmt.Fprintf(w, "hello %s. Age is %s", msg, age)
	}
}

func (m myAPI) login(secretKey string) http.HandlerFunc {
	tmpl, err := template.New("myTpl").Parse(`<!DOCTYPE html>
<html>
<head>
<link rel="stylesheet" href="https://unpkg.com/mvp.css@1.12/mvp.css">
<style>
	:root{
		/* ovverive variables from mvp.css */
		--line-height: 1.0;
		--font-family: system-ui;
	}
	html {
	/*
	from:
	  - https://www.swyx.io/css-100-bytes
	  - https://news.ycombinator.com/item?id=32972768
	  - https://github.com/andybrewer/mvp
	  - https://www.joshwcomeau.com/css/full-bleed/
	  - https://www.joshwcomeau.com/css/custom-css-reset/
	*/
		max-width: 70ch;
		padding: 3em 1em;
		margin: auto;
		font-size: 1.0em;
	}
</style>
</head>
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

	type User struct {
		Email string
		Name  string
	}

	return func(w http.ResponseWriter, r *http.Request) {
		reqL := m.l.WithCtx(r.Context())

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

		email := r.FormValue("email")
		firstName := r.FormValue("firstName")

		u := &User{Email: email, Name: firstName}

		s, errM := json.Marshal(u)
		if errM != nil {
			panic(errM)
		}

		cookieName := "ong_example_session_cookie"
		c, errM := cookie.GetEncrypted(r, cookieName, secretKey)
		reqL.WithImmediate().Info(log.F{
			"msg":    "login handler log cookie",
			"err":    errM,
			"cookie": c,
		})

		cookie.SetEncrypted(
			r,
			w,
			cookieName,
			string(s),
			"localhost",
			23*24*time.Hour,
			secretKey,
		)

		_, _ = fmt.Fprintf(w, "you have submitted: %s", r.Form)
	}
}
