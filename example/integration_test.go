package main_test

import (
	"net/http"
	"testing"

	"github.com/akshayjshah/attest"
)

// This tests depend on the functionality in the /example folder.
// go:build integration // TODO:
func TestIntegration(t *testing.T) {
	t.Parallel()

	t.Run("https_redirection", func(t *testing.T) {
		t.Parallel()

		{
			c := &http.Client{
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}
			res, err := c.Get("http://127.0.0.1:65080/health")
			attest.Ok(t, err)
			defer res.Body.Close()
			attest.Equal(t, res.StatusCode, http.StatusPermanentRedirect)
		}

		{
			c := &http.Client{}
			res, err := c.Get("https://localhost:65081/health")
			attest.Ok(t, err)
			defer res.Body.Close()
			attest.Equal(t, res.StatusCode, http.StatusOK)
		}
	})

	t.Run("pprof", func(t *testing.T) {
		t.Parallel()

		c := &http.Client{}
		res, err := c.Get("http://127.0.0.1:65060/debug/pprof/profile?seconds=3")
		attest.Ok(t, err)
		defer res.Body.Close()
		attest.Equal(t, res.StatusCode, http.StatusOK)
	})

	t.Run("static_file_server", func(t *testing.T) {
		t.Parallel()

		url := "https://localhost:65081/staticAssets/hello.css"
		{
			c := &http.Client{}
			req, err := http.NewRequest(http.MethodGet, url, nil)
			attest.Ok(t, err)
			res, err := c.Do(req)
			attest.Ok(t, err)
			defer res.Body.Close()
			attest.Equal(t, res.StatusCode, http.StatusUnauthorized)
		}
		{
			c := &http.Client{}
			req, err := http.NewRequest(http.MethodGet, url, nil)
			attest.Ok(t, err)
			req.SetBasicAuth("user", "some-long-passwd")

			res, err := c.Do(req)
			attest.Ok(t, err)
			defer res.Body.Close()
			attest.Equal(t, res.StatusCode, http.StatusOK)
		}
	})

	t.Run("check_age", func(t *testing.T) {
		t.Parallel()

		c := &http.Client{}
		res, err := c.Get("https://localhost:65081/check/67")
		attest.Ok(t, err)
		defer res.Body.Close()
		attest.Equal(t, res.StatusCode, http.StatusOK)
	})

	t.Run("login", func(t *testing.T) {
		t.Parallel()

		{
			c := &http.Client{}
			res, err := c.Get("https://localhost:65081/login")
			attest.Ok(t, err)
			defer res.Body.Close()
			attest.Equal(t, res.StatusCode, http.StatusOK)
		}

		{ // with slash suffix
			c := &http.Client{}
			res, err := c.Get("https://localhost:65081/login/")
			attest.Ok(t, err)
			defer res.Body.Close()
			attest.Equal(t, res.StatusCode, http.StatusOK)
		}
	})

	t.Run("panic", func(t *testing.T) {
		t.Parallel()

		c := &http.Client{}
		res, err := c.Get("https://localhost:65081/panic")
		attest.Ok(t, err)
		defer res.Body.Close()
		attest.Equal(t, res.StatusCode, http.StatusInternalServerError)
	})
}
