package client

import (
	"net/http"
	"strings"
	"testing"

	"github.com/akshayjshah/attest"
)

func TestClient(t *testing.T) {
	t.Parallel()

	// aws metadata api.
	urlsInPrivate := []string{
		"http://[fd00:ec2::254]/latest/meta-data/",
		// "http://169.254.169.254/latest/meta-data/",
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

		ssrfSafe := true

		for _, url := range urlsInPrivate {
			cli := New(ssrfSafe)
			res, err := cli.Get(url)
			attest.Error(t, err)
			clean(res)
			attest.Subsequence(t, err.Error(), "is not a public IP address")
		}

		for _, url := range urlsInPublic {
			cli := New(ssrfSafe)
			res, err := cli.Get(url)
			attest.Ok(t, err)
			clean(res)
			attest.Equal(t, res.StatusCode, http.StatusOK)
		}
	})

	t.Run("ssrf unsafe", func(t *testing.T) {
		t.Parallel()

		ssrfSafe := false

		for _, url := range urlsInPrivate {
			cli := New(ssrfSafe)
			res, err := cli.Get(url)
			attest.Error(t, err)
			clean(res)
			attest.False(t, strings.Contains(err.Error(), "is not a public IP address"))
		}

		for _, url := range urlsInPublic {
			cli := New(ssrfSafe)
			res, err := cli.Get(url)
			attest.Ok(t, err)
			clean(res)
			attest.Equal(t, res.StatusCode, http.StatusOK)
		}
	})
}
