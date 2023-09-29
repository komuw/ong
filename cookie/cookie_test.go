package cookie

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/komuw/ong/internal/tst"
	"go.akshayshah.org/attest"
	"go.uber.org/goleak"
)

func setHandler(name, value, domain string, mAge time.Duration, jsAccess bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		Set(w, name, value, domain, mAge, jsAccess)
		fmt.Fprint(w, "hello")
	}
}

func setEncryptedHandler(name, value, domain string, mAge time.Duration, secretKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		SetEncrypted(r, w, name, value, domain, mAge, secretKey)
		fmt.Fprint(w, "hello")
	}
}

func TestMain(m *testing.M) {
	// call flag.Parse() here if TestMain uses flags
	goleak.VerifyTestMain(m)
}

func TestCookies(t *testing.T) {
	t.Parallel()

	t.Run("set succeds", func(t *testing.T) {
		t.Parallel()

		name := "logId"
		value := "skmHajue8k"
		domain := "localhost"
		mAge := 1 * time.Minute
		handler := setHandler(name, value, domain, mAge, false)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		handler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, len(res.Cookies()), 1)
		attest.Equal(t, res.Cookies()[0].Name, name)

		cookie := res.Cookies()[0]
		now := time.Now()

		attest.True(t, cookie.MaxAge >= 1)
		attest.True(t, cookie.Expires.Sub(now) > 1)
		attest.Equal(t, cookie.HttpOnly, true)
	})

	t.Run("session cookie", func(t *testing.T) {
		t.Parallel()

		name := "logId"
		value := "skmHajue8k"
		domain := "localhost"
		mAge := 0 * time.Minute
		handler := setHandler(name, value, domain, mAge, false)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		handler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, len(res.Cookies()), 1)
		attest.Equal(t, res.Cookies()[0].Name, name)

		cookie := res.Cookies()[0]
		attest.Equal(t, cookie.MaxAge, 0)
		attest.Equal(t, cookie.Expires, time.Time{})
		attest.Equal(t, cookie.HttpOnly, true)
	})

	t.Run("js accesible cookie", func(t *testing.T) {
		t.Parallel()

		name := "csrf"
		value := "skmHajue8k"
		domain := "localhost"
		mAge := 1 * time.Minute
		jsAccess := true
		handler := setHandler(name, value, domain, mAge, jsAccess)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		handler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, len(res.Cookies()), 1)
		attest.Equal(t, res.Cookies()[0].Name, name)

		cookie := res.Cookies()[0]
		now := time.Now()

		attest.True(t, cookie.MaxAge >= 1)
		attest.True(t, cookie.Expires.Sub(now) > 1)
		attest.Equal(t, cookie.HttpOnly, false)
	})

	t.Run("set encrypted", func(t *testing.T) {
		t.Parallel()

		name := "logId"
		value := "hello world are you okay"
		domain := "localhost"
		mAge := 23 * time.Hour
		secretKey := tst.SecretKey()
		handler := setEncryptedHandler(name, value, domain, mAge, secretKey)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		handler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, len(res.Cookies()), 1)
		attest.Equal(t, res.Cookies()[0].Name, name)

		cookie := res.Cookies()[0]
		now := time.Now()

		attest.True(t, cookie.MaxAge >= 1)
		attest.True(t, cookie.Expires.Sub(now) > 1)
		attest.Equal(t, cookie.HttpOnly, true)

		req.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
		val, err := GetEncrypted(req, cookie.Name, secretKey)

		attest.Ok(t, err)
		attest.Equal(t, val.Value, value)
		attest.Equal(t, val.Name, cookie.Name)
	})

	t.Run("encrypted expired", func(t *testing.T) {
		t.Parallel()

		name := "logId"
		value := "hello world are you okay"
		domain := "localhost"
		mAge := -23 * time.Hour
		secretKey := tst.SecretKey()
		handler := setEncryptedHandler(name, value, domain, mAge, secretKey)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		handler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, len(res.Cookies()), 1)
		attest.Equal(t, res.Cookies()[0].Name, name)

		cookie := res.Cookies()[0]

		attest.Equal(t, cookie.HttpOnly, true)

		req.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
		val, err := GetEncrypted(req, cookie.Name, secretKey)

		attest.Zero(t, val)
		attest.Error(t, err)
	})

	t.Run("anti-replay", func(t *testing.T) {
		t.Parallel()

		header := "Anti-Replay"
		antiReplayFunc := func(r *http.Request) string {
			return r.Header.Get(header)
		}

		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		headerVal := "some-unique-val-1326"
		req.Header.Add(header, headerVal)
		req = SetAntiReplay(req, antiReplayFunc(req))

		res := getAntiReplay(req)
		attest.Equal(t, res, headerVal)
	})
}

func deleteHandler(name, value, domain string, mAge time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		Set(w, name, value, domain, mAge, false)
		Delete(w, name, domain)
		fmt.Fprint(w, "hello")
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()

	t.Run("delete", func(t *testing.T) {
		t.Parallel()

		name := "logId"
		domain := "localhost"
		value := "skmHajue8k"
		mAge := 1 * time.Minute
		rec := httptest.NewRecorder()
		handler := deleteHandler(name, value, domain, mAge)

		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		handler.ServeHTTP(rec, req)
		res := rec.Result()
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, len(res.Cookies()), 2) // deleting cookies is done by appending to existing cookies.

		cookie := res.Cookies()[1]
		attest.True(t, cookie.MaxAge < 0)
	})
}

var result int //nolint:gochecknoglobals

func BenchmarkSetEncrypted(b *testing.B) {
	var r int
	req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
	res := httptest.NewRecorder()

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		r = testSetEncrypted(req, res)
	}

	// always store the result to a package level variable
	// so the compiler cannot eliminate the Benchmark itself.
	result = r
}

func testSetEncrypted(req *http.Request, res http.ResponseWriter) int {
	SetEncrypted(req, res, "name", "value", "example.com", 2*time.Hour, tst.SecretKey())
	return 3
}
