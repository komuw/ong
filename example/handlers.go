package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/komuw/ong/client"
	"github.com/komuw/ong/cookie"
	"github.com/komuw/ong/cry"
	"github.com/komuw/ong/errors"
	"github.com/komuw/ong/id"
	"github.com/komuw/ong/log"
	"github.com/komuw/ong/middleware"
	"github.com/komuw/ong/mux"
	"github.com/komuw/ong/sess"
	"github.com/komuw/ong/xcontext"

	"golang.org/x/exp/slog"
)

// db is a dummy database.
type db interface {
	Get(key string) string
	Set(key, value string)
}

// app represents component as a struct, shared dependencies as fields, no global state.
type app struct {
	db db
	l  *slog.Logger
}

// NewApp creates a new app.
func NewApp(d db, l *slog.Logger) app {
	return app{
		db: d,
		l:  l,
	}
}

// health handler showcases the use of:
// - encryption/decryption.
// - random id.
func (a app) health(secretKey string) http.HandlerFunc {
	var (
		once        sync.Once
		serverBoot  time.Time = time.Now().UTC()
		serverStart time.Time
		serverID    = id.New()
		enc         = cry.New(secretKey)
	)

	return func(w http.ResponseWriter, r *http.Request) {
		// intialize somethings only once for perfomance.
		once.Do(func() {
			serverStart = time.Now().UTC()
		})

		encryptedSrvID := enc.EncryptEncode(serverID)

		res := fmt.Sprintf("serverBoot=%s, serverStart=%s, serverId=%s\n", serverBoot, serverStart, encryptedSrvID)
		_, _ = io.WriteString(w, res)
	}
}

// check handler showcases the use of:
// - mux.Param.
// - sessions.
// - xcontext.Detach.
// - safe http client.
// - error wrapping.
func (a app) check(msg string) http.HandlerFunc {
	cli := client.Safe(a.l)

	return func(w http.ResponseWriter, r *http.Request) {
		cartID := "afakHda8eqL"

		age := mux.Param(r.Context(), "age")
		sess.SetM(r, sess.M{
			"name":    "John Doe",
			"age":     age,
			"cart_id": cartID,
		})

		if sess.Get(r, "cart_id") != "" {
			if sess.Get(r, "cart_id") != cartID {
				http.Error(w, "wrong cartID", http.StatusBadRequest)
				return
			}
		}

		go func(ctx context.Context) {
			ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()

			makeReq := func(url string) (code int, errp error) {
				defer errors.Dwrap(&errp)

				req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
				if err != nil {
					return 0, err
				}
				resp, err := cli.Do(req)
				if err != nil {
					return 0, err
				}
				defer func() { _ = resp.Body.Close() }()

				return resp.StatusCode, nil
			}

			l := log.WithID(ctx, a.l)
			code, err := makeReq("https://example.com")
			if err != nil {
				l.Error("handler error", err)
			}
			l.Info("req succeded", "code", code)
		}(
			// we need to detach context,
			// since this goroutine can outlive the http request lifecycle.
			xcontext.Detach(r.Context()),
		)

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
	<label>First Name:</label><br>
	<input type="text" id="firstName" name="firstName"><br>
	<label>Email:</label><br>
	<input type="text" id="email" name="email"><br>
	<label>Password:</label><br>
	<input type="password" id="password" name="password"><br>
	
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
		reqL := log.WithID(r.Context(), a.l)

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

			if errE := tmpl.Execute(w, data); errE != nil {
				http.Error(w, errE.Error(), http.StatusInternalServerError)
				return
			}
			return
		}

		if errP := r.ParseForm(); errP != nil {
			http.Error(w, errP.Error(), http.StatusInternalServerError)
			return
		}

		email := r.FormValue("email")
		firstName := r.FormValue("firstName")
		password := r.FormValue("password")

		u := &User{Email: email, Name: firstName}

		s, errM := json.Marshal(u)
		if errM != nil {
			http.Error(w, errM.Error(), http.StatusInternalServerError)
			return
		}

		cookieName := "example_session_cookie"
		c, errM := cookie.GetEncrypted(r, cookieName, secretKey)
		reqL.Info("login handler log cookie",
			"err", errM,
			"cookie", c,
		)

		cookie.SetEncrypted(
			r,
			w,
			cookieName,
			string(s),
			"localhost",
			23*24*time.Hour,
			secretKey,
		)

		existingPasswdHash := a.db.Get("passwd")
		if e := cry.Eql(password, existingPasswdHash); e != nil {
			// passwd did not exist before.
			hashedPasswd, errH := cry.Hash(password)
			if errH != nil {
				http.Error(w, errH.Error(), http.StatusInternalServerError)
				return
			}
			a.db.Set("passwd", hashedPasswd)
		}

		_, _ = fmt.Fprintf(w, "you have submitted: %s", r.Form)
	}
}

// handleFileServer handler showcases the use of:
// - middleware.ClientIP
// - middleware.ClientFingerPrint
// - logging
func (a app) handleFileServer() http.HandlerFunc {
	// Do NOT let `http.FileServer` be able to serve your root directory.
	// Otherwise, your .git folder and other sensitive info(including http://localhost:65080/main.go) may be available
	// instead create a folder that only has your templates and server that.

	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	cwd = strings.TrimSuffix(cwd, "example")
	dir := filepath.Join(cwd, "/example/staticAssets")

	fs := http.FileServer(http.Dir(dir))
	realHandler := http.StripPrefix("/staticAssets/", fs).ServeHTTP
	return func(w http.ResponseWriter, r *http.Request) {
		reqL := log.WithID(r.Context(), a.l)
		reqL.Info("handleFileServer", "clientIP", middleware.ClientIP(r), "clientFingerPrint", middleware.ClientFingerPrint(r))

		slog.NewLogLogger(reqL.Handler(), log.LevelImmediate).
			Println("this is now a Go standard library logger")

		realHandler(w, r)
	}
}

// panic handler showcases the use of:
// - recoverer middleware.
func (a app) panic() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		names := []string{"John", "Jane", "Kamau"}
		_ = 93
		msg := "hey"
		n := names[934]
		_, _ = io.WriteString(w, fmt.Sprintf("%s %s", msg, n))
	}
}
