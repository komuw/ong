package client

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/komuw/ong/log"

	"github.com/akshayjshah/attest"
)

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

		cli := SafeClient()

		for _, url := range urlsInPrivate {
			res, err := cli.Get(url)
			attest.Error(t, err)
			clean(res)
			attest.Subsequence(t, err.Error(), "is not a public IP address")
		}

		for _, url := range urlsInPublic {
			res, err := cli.Get(url)
			attest.Ok(t, err)
			clean(res)
			attest.Equal(t, res.StatusCode, http.StatusOK, attest.Sprintf("url=%s", url))
		}
	})

	t.Run("ssrf unsafe", func(t *testing.T) {
		t.Parallel()

		cli := UnsafeClient()

		for _, url := range urlsInPrivate {
			if strings.Contains(url, "169.254.169.254") {
				// the following IP when run from laptop resolves to IP of wifi router.
				// Thus we have to disable it from test, since the test tries making a request to the router
				// and gets a 404.
				break
			}
			res, err := cli.Get(url)
			attest.Error(t, err)
			clean(res)
			attest.False(t, strings.Contains(err.Error(), "is not a public IP address"))
		}

		for _, url := range urlsInPublic {
			res, err := cli.Get(url)
			attest.Ok(t, err)
			clean(res)
			attest.Equal(t, res.StatusCode, http.StatusOK, attest.Sprintf("url=%s", url))
		}
	})
}

// TODO: fix name.
func TestTodo(t *testing.T) {
	t.Parallel()

	t.Run("GET", func(t *testing.T) {
		t.Parallel()

		{
			// error
			w := &bytes.Buffer{}
			maxMsgs := 15
			ctx := context.Background()
			l := log.New(ctx, w, maxMsgs)

			cli := newClient(true, l)

			res, err := cli.Get(ctx, "https://ajmsmsYnns.com")

			attest.Zero(t, res)
			attest.Error(t, err)
			attest.NotZero(t, w.String())
			attest.Subsequence(t, w.String(), "error")
		}

		{
			// success
			w := &bytes.Buffer{}
			maxMsgs := 15
			ctx := context.Background()
			l := log.New(ctx, w, maxMsgs)

			cli := newClient(true, l)

			res, err := cli.Get(ctx, "https://example.com")

			attest.NotZero(t, res)
			attest.Ok(t, err)
			attest.Zero(t, w.String())
			attest.Equal(t, res.StatusCode, http.StatusOK)
		}
	})
}
