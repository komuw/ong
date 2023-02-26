package client

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/akshayjshah/attest"
	"github.com/komuw/ong/log"
	"golang.org/x/exp/slog"
)

func getLogger() *slog.Logger {
	w := &bytes.Buffer{}
	maxMsgs := 15
	return log.New(w, maxMsgs)(context.Background())
}

// TODO: (komuw)
// func TestMain(m *testing.M) {
// 	// call flag.Parse() here if TestMain uses flags
// 	goleak.VerifyTestMain(m)
// }

func TestClient(t *testing.T) {
	t.Parallel()

	// aws metadata api.
	urlsInPrivate := []string{
		"http://[fd00:ec2::254]/latest/meta-data/",
		"http://169.254.169.254/latest/meta-data/",
	}

	urlsInPublic := []string{
		"http://www.example.com",
		"https://www.example.com",
	}

	clean := func(res *http.Response) {
		t.Cleanup(func() {
			if res != nil {
				res.Body.Close()
			}
		})
	}

	t.Run("ssrf safe", func(t *testing.T) {
		t.Parallel()

		cli := Safe(getLogger())

		for _, url := range urlsInPrivate {
			res, err := cli.Get(url) // nolint:bodyclose
			clean(res)
			attest.Error(t, err)
			attest.True(t, strings.Contains(err.Error(), errPrefix))
		}

		for _, url := range urlsInPublic {
			res, err := cli.Get(url) // nolint:bodyclose
			clean(res)
			attest.Ok(t, err)
			attest.Equal(t, res.StatusCode, http.StatusOK, attest.Sprintf("url=%s", url))
		}
	})

	t.Run("ssrf unsafe", func(t *testing.T) {
		t.Parallel()

		cli := Unsafe(getLogger())

		for _, url := range urlsInPrivate {
			if strings.Contains(url, "169.254.169.254") {
				// the following IP when run from laptop resolves to IP of wifi router.
				// Thus we have to disable it from test, since the test tries making a request to the router
				// and gets a 404.
				break
			}
			res, err := cli.Get(url) // nolint:bodyclose
			clean(res)
			attest.Error(t, err)
			attest.False(t, strings.Contains(err.Error(), errPrefix))
		}

		for _, url := range urlsInPublic {
			res, err := cli.Get(url) // nolint:bodyclose
			clean(res)
			attest.Ok(t, err)
			attest.Equal(t, res.StatusCode, http.StatusOK, attest.Sprintf("url=%s", url))
		}
	})

	t.Run("GET", func(t *testing.T) {
		t.Parallel()

		{
			// error
			w := &bytes.Buffer{}
			maxMsgs := 15
			l := log.New(w, maxMsgs)(context.Background())

			cli := Safe(l)

			res, err := cli.Get("https://ajmsmsYnns.com") // nolint:bodyclose
			clean(res)
			attest.Zero(t, res)
			attest.Error(t, err)
			attest.NotZero(t, w.String())
			attest.Subsequence(t, w.String(), "ERROR")
		}

		{
			// success
			w := &bytes.Buffer{}
			maxMsgs := 15
			l := log.New(w, maxMsgs)(context.Background())

			cli := Safe(l)

			res, err := cli.Get("https://example.com") // nolint:bodyclose
			clean(res)
			attest.NotZero(t, res)
			attest.Ok(t, err)
			attest.Zero(t, w.String())
			attest.Equal(t, res.StatusCode, http.StatusOK)
		}
	})

	t.Run("POST", func(t *testing.T) {
		t.Parallel()

		{
			// error
			w := &bytes.Buffer{}
			maxMsgs := 15
			l := log.New(w, maxMsgs)(context.Background())

			cli := Safe(l)

			b := strings.NewReader(`{"key":"value"}`)
			res, err := cli.Post("https://ajmsmsYnns.com", "application/json", b) // nolint:bodyclose
			clean(res)
			attest.Zero(t, res)
			attest.Error(t, err)
			attest.NotZero(t, w.String())
			attest.Subsequence(t, w.String(), "ERROR")
		}

		{
			// success
			w := &bytes.Buffer{}
			maxMsgs := 15
			l := log.New(w, maxMsgs)(context.Background())

			cli := Safe(l)

			b := strings.NewReader(`{"key":"value"}`)
			res, err := cli.Post("https://example.com", "application/json", b) // nolint:bodyclose
			clean(res)
			attest.NotZero(t, res)
			attest.Ok(t, err)
			attest.Zero(t, w.String())
			attest.Equal(t, res.StatusCode, http.StatusOK)
		}
	})
}
