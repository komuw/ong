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

	"github.com/komuw/ong/client"
	"github.com/komuw/ong/cookie"
	"github.com/komuw/ong/errors"
	"github.com/komuw/ong/id"
	"github.com/komuw/ong/log"
	"github.com/komuw/ong/middleware"
	"github.com/komuw/ong/mux"
	"github.com/komuw/ong/sess"
)

// TODO: remove this checklist
/*
Things we need to showcase.
1. csrf tokens.                   - DONE(login)
2. csp tokens.                    - DONE(login)
3. encrypted cookies(get & set)   - DONE(login)
4. use of safe http client.       - DONE(health)
5. encryption/decryption.         - DONE(health)  TODO:
6. Hashing passwords.             - DONE(login)
7. error wrapping.                 - DONE(health)
8. error Dwrap                     - DONE(health)
9. id.New()                        - DONE(health)
10. logging.                       - DONE(handleFileServer)
    - WithCtx                      - DONE(handleFileServer)
	- stdlib.                      - DONE(handleFileServer)
11. middleware.clientIp            - DONE(handleFileServer)
12. middleware.ClientIPstrategy    - DONE(main func.)
13. mux.Param                      - DONE(check)
14. session.Get and set.           - DONE(check)
15. xcontext.Detach                - DONE(check)
*/

// app represents component as a struct, shared dependencies as fields, no global state.
type app struct {
	db string
	l  log.Logger
}

// NewApp creates a new app.
func NewApp(db string) app {
	return app{
		db: db,
		l:  log.New(os.Stdout, 1000),
	}
}

// health handler showcases the use of:
// - safe http client.
// - encryption/decryption.
// - error wrapping.
// - random id.
func (a app) health() http.HandlerFunc {
	var (
		once        sync.Once
		serverBoot  time.Time = time.Now().UTC()
		serverStart time.Time
		cli         = client.Safe(a.l)
		serverID    = id.New()
	)

	return func(w http.ResponseWriter, r *http.Request) {
		// intialize somethings only once for perfomance.
		once.Do(func() {
			serverStart = time.Now().UTC()
		})

		makeReq := func(url string) (code int, errp error) {
			defer errors.Dwrap(&errp)

			req, err := http.NewRequestWithContext(r.Context(), "GET", url, nil)
			if err != nil {
				return 0, err
			}
			resp, err := cli.Do(req)
			if err != nil {
				return 0, err
			}
			defer resp.Body.Close()

			return resp.StatusCode, nil
		}

		code, err := makeReq("https://example.com")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		res := fmt.Sprintf("serverBoot=%s, serverStart=%s, serverId=%s, statusCode=%d\n", serverBoot, serverStart, serverID, code)
		_, _ = io.WriteString(w, res)
	}
}

// check handler showcases the use of:
// - mux.Param
// - sessions
// - xcontext.Detach
func (a app) check(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reqL := a.l.WithCtx(r.Context())

		cartID := "afakHda8eqL"
		sess.SetM(r, sess.M{
			"name":    "John Doe",
			"age":     "88",
			"cart_id": cartID,
		})

		reqL.WithImmediate().Info(log.F{"cart_id": sess.Get(r, "cart_id")})
		if sess.Get(r, "cart_id") != "" {
			if sess.Get(r, "cart_id") != cartID {
				http.Error(w, "wrong cartID", http.StatusBadRequest)
				return
			}
		}

		age := mux.Param(r.Context(), "age")
		// use msg, which is a dependency specific to this handler
		_, _ = fmt.Fprintf(w, "hello %s. Age is %s", msg, age)
	}
}

// login handler showcases the use of:
// - csrf tokens.
// - csp tokens.
// - encrypted cookies
// - hashing passwords.
func (a app) login(secretKey string) http.HandlerFunc {
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
		reqL := a.l.WithCtx(r.Context())

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
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			return
		}

		if err = r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		email := r.FormValue("email")
		firstName := r.FormValue("firstName")

		u := &User{Email: email, Name: firstName}

		s, errM := json.Marshal(u)
		if errM != nil {
			http.Error(w, errM.Error(), http.StatusInternalServerError)
			return
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

// handleFileServer handler showcases the use of:
// - middleware.ClientIP
// - logging
func (a app) handleFileServer() http.HandlerFunc {
	// Do NOT let `http.FileServer` be able to serve your root directory.
	// Otherwise, your .git folder and other sensitive info(including http://localhost:65080/main.go) may be available
	// instead create a folder that only has your templates and server that.
	fs := http.FileServer(http.Dir("./stuff"))
	realHandler := http.StripPrefix("somePrefix", fs).ServeHTTP
	return func(w http.ResponseWriter, r *http.Request) {
		reqL := a.l.WithCtx(r.Context())

		reqL.Info(log.F{"msg": "handleFileServer", "clientIP": middleware.ClientIP(r)})

		reqL.StdLogger().Println("this is now a Go standard library logger")

		realHandler(w, r)
	}
}
