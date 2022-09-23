package client

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/komuw/ong/log"

	"github.com/akshayjshah/attest"
)

func getLogger(ctx context.Context) log.Logger {
	w := &bytes.Buffer{}
	maxMsgs := 15
	return log.New(ctx, w, maxMsgs)
}

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

		ctx := context.Background()
		cli := SafeClient(getLogger(ctx))

		for _, url := range urlsInPrivate {
			res, err := cli.Get(ctx, url) // nolint:bodyclose
			clean(res)
			attest.Error(t, err)
			attest.True(t, strings.Contains(err.Error(), errPrefix))
		}

		for _, url := range urlsInPublic {
			res, err := cli.Get(ctx, url) // nolint:bodyclose
			clean(res)
			attest.Ok(t, err)
			attest.Equal(t, res.StatusCode, http.StatusOK, attest.Sprintf("url=%s", url))
		}
	})

	t.Run("ssrf unsafe", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		cli := UnsafeClient(getLogger(ctx))

		for _, url := range urlsInPrivate {
			res, err := cli.Get(ctx, url) // nolint:bodyclose
			clean(res)
			attest.Error(t, err)
			attest.False(t, strings.Contains(err.Error(), errPrefix))
		}

		for _, url := range urlsInPublic {
			res, err := cli.Get(ctx, url) // nolint:bodyclose
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
			ctx := context.Background()
			l := log.New(ctx, w, maxMsgs)

			cli := SafeClient(l)

			res, err := cli.Get(ctx, "https://ajmsmsYnns.com") // nolint:bodyclose
			clean(res)
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

			cli := SafeClient(l)

			res, err := cli.Get(ctx, "https://example.com") // nolint:bodyclose
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
			ctx := context.Background()
			l := log.New(ctx, w, maxMsgs)

			cli := SafeClient(l)

			b := strings.NewReader(`{"key":"value"}`)
			res, err := cli.Post(ctx, "https://ajmsmsYnns.com", "application/json", b) // nolint:bodyclose
			clean(res)
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

			cli := SafeClient(l)

			b := strings.NewReader(`{"key":"value"}`)
			res, err := cli.Post(ctx, "https://example.com", "application/json", b) // nolint:bodyclose
			clean(res)
			attest.NotZero(t, res)
			attest.Ok(t, err)
			attest.Zero(t, w.String())
			attest.Equal(t, res.StatusCode, http.StatusOK)
		}
	})
}
