package middleware

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/komuw/ong/config"
	"github.com/komuw/ong/cookie"
	"github.com/komuw/ong/internal/tst"
	"github.com/komuw/ong/sess"

	"go.akshayshah.org/attest"
)

func bigMap() map[string]string {
	y := map[string]string{}
	for i := 0; i < 100; i++ {
		k := fmt.Sprintf("key:%d", i)
		v := fmt.Sprintf("val:%d", i)
		y[k] = v
	}
	return y
}

func someSessionHandler(msg, key, value string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess.Set(r, key, value)
		sess.SetM(r, bigMap())
		fmt.Fprint(w, msg)
	}
}

// See https://github.com/komuw/ong/issues/205
func templateVarsHandler(t *testing.T, name string) http.HandlerFunc {
	tmpl, err := template.New("myTpl").Parse(`<!DOCTYPE html>
<html>
<head>
</head>
<body>
	<h2>Welcome to awesome website {{.Name}}.</h2>
</body>
</html>`)
	if err != nil {
		t.Fatal(err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		sess.Set(r, "name", name)

		data := struct {
			Name string
		}{Name: name}
		if err = tmpl.Execute(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func TestSession(t *testing.T) {
	t.Parallel()

	t.Run("middleware succeds", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		secretKey := tst.SecretKey()
		domain := "localhost"
		key := "name"
		value := "John Doe"
		wrappedHandler := session(
			someSessionHandler(msg, key, value),
			secretKey,
			domain,
			config.DefaultSessionCookieDuration,
			func(r http.Request) string { return r.RemoteAddr },
		)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)
	})

	t.Run("middleware set succeds", func(t *testing.T) {
		t.Parallel()

		msg := "hello world wide."
		secretKey := tst.SecretKey()
		domain := "localhost"
		key := "name"
		value := "John Doe"

		header := "Anti-Replay"
		antiReplayFunc := func(r http.Request) string {
			return r.Header.Get(header)
		}
		wrappedHandler := session(
			someSessionHandler(msg, key, value),
			secretKey,
			domain,
			config.DefaultSessionCookieDuration,
			antiReplayFunc,
		)

		ts := httptest.NewServer(
			wrappedHandler,
		)
		t.Cleanup(func() {
			ts.Close()
		})

		req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
		attest.Ok(t, err)
		headerVal := "some-unique-val-1326"
		req.Header.Add(header, headerVal)

		res, err := ts.Client().Do(req)
		attest.Ok(t, err)
		t.Cleanup(func() {
			res.Body.Close()
		})

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)

		attest.Equal(t, len(res.Cookies()), 1)
		attest.Equal(t, res.Cookies()[0].Name, sess.CookieName)
		attest.NotZero(t, res.Cookies()[0].Value)

		{
			req2, errN := http.NewRequest(http.MethodGet, ts.URL, nil)
			attest.Ok(t, errN)

			// very important to do this assignment, since `antiReplayFunc` checks for IP mismatch.
			req2.Header.Add(header, headerVal)
			req2 = cookie.SetAntiReplay(req2, antiReplayFunc(*req2))
			attest.Ok(t, err)
			req2.AddCookie(&http.Cookie{
				Name:  res.Cookies()[0].Name,
				Value: res.Cookies()[0].Value,
			})

			c, errG := cookie.GetEncrypted(req2, sess.CookieName, secretKey)
			attest.Ok(t, errG)
			attest.Subsequence(t, c.Value, key)
			attest.Subsequence(t, c.Value, value)
		}
	})

	t.Run("with template variables", func(t *testing.T) {
		t.Parallel()

		secretKey := tst.SecretKey()
		domain := "localhost"
		name := "John Doe"
		wrappedHandler := session(
			templateVarsHandler(t, name),
			secretKey,
			domain,
			config.DefaultSessionCookieDuration,
			func(r http.Request) string { return r.RemoteAddr },
		)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Subsequence(t, string(rb), name)

		attest.Equal(t, len(res.Cookies()), 1)
		attest.Equal(t, res.Cookies()[0].Name, sess.CookieName)
		attest.NotZero(t, res.Cookies()[0].Value)
	})

	t.Run("anti replay", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		secretKey := tst.SecretKey()
		domain := "localhost"
		key := "name"
		value := "John Doe"

		antiReplayFunc := func(r http.Request) string { return r.RemoteAddr }
		wrappedHandler := session(
			someSessionHandler(msg, key, value),
			secretKey,
			domain,
			config.DefaultSessionCookieDuration,
			antiReplayFunc,
		)

		ip1 := "128.45.2.3"
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.RemoteAddr = ip1
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)

		{ // Success.
			req2 := httptest.NewRequest(http.MethodGet, "/hey-uri", nil)
			// very important to do this assignment, since `antiReplayFunc` checks for IP mismatch.
			req2.RemoteAddr = ip1
			req2 = cookie.SetAntiReplay(req2, antiReplayFunc(*req2))
			req2.AddCookie(&http.Cookie{
				Name:  res.Cookies()[0].Name,
				Value: res.Cookies()[0].Value,
			})

			c, errG := cookie.GetEncrypted(req2, sess.CookieName, secretKey)
			attest.Ok(t, errG)
			attest.Subsequence(t, c.Value, key)
			attest.Subsequence(t, c.Value, value)
		}

		{ // Failure.
			req3 := httptest.NewRequest(http.MethodGet, "/hey-uri", nil)
			ip2 := "148.65.4.3"
			req3.RemoteAddr = ip2
			req3 = cookie.SetAntiReplay(req3, antiReplayFunc(*req3))
			req3.AddCookie(&http.Cookie{
				Name:  res.Cookies()[0].Name,
				Value: res.Cookies()[0].Value,
			})

			c, errG := cookie.GetEncrypted(req3, sess.CookieName, secretKey)
			attest.Error(t, errG)
			attest.Zero(t, c)
			attest.Subsequence(t, errG.Error(), "mismatched anti replay value")
		}
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		secretKey := tst.SecretKey()
		domain := "localhost"
		key := "bothNames"
		value := "John Doe Jnr"
		wrappedHandler := session(
			someSessionHandler(msg, key, value),
			secretKey,
			domain,
			config.DefaultSessionCookieDuration,
			func(r http.Request) string { return r.RemoteAddr },
		)

		runhandler := func() {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), msg)
		}

		wg := &sync.WaitGroup{}
		for rN := 0; rN <= 10; rN++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				runhandler()
			}()
		}
		wg.Wait()
	})
}
