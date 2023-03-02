// go:build integration
package main_test

import (
	"net/http"
	"testing"

	"github.com/akshayjshah/attest"
)

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
}
