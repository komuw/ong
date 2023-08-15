//go:build integration
// +build integration

package main_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	vegeta "github.com/tsenart/vegeta/v12/lib"
	"go.akshayshah.org/attest"
)

// This tests depend on the functionality in the /example folder.
func TestIntegration(t *testing.T) {
	// This tests should not run in parallel so as not to affect each other.
	// t.Parallel()

	t.Run("https_redirection", func(t *testing.T) {
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
		c := &http.Client{}
		res, err := c.Get("http://127.0.0.1:65079/debug/pprof/profile?seconds=3")
		attest.Ok(t, err)
		defer res.Body.Close()
		attest.Equal(t, res.StatusCode, http.StatusOK)
	})

	t.Run("static_file_server", func(t *testing.T) {
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
		c := &http.Client{}
		res, err := c.Get("https://localhost:65081/check/67")
		attest.Ok(t, err)
		defer res.Body.Close()
		attest.Equal(t, res.StatusCode, http.StatusOK)
	})

	t.Run("login", func(t *testing.T) {
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
		c := &http.Client{}
		res, err := c.Get("https://localhost:65081/panic")
		attest.Ok(t, err)
		defer res.Body.Close()
		attest.Equal(t, res.StatusCode, http.StatusInternalServerError)
	})

	t.Run("rate_limit_test", func(t *testing.T) {
		rate := vegeta.Rate{
			// this rate of 90/sec is less than the rateLimit used by ong of 100/sec
			// https://github.com/komuw/ong/blob/v0.0.42/middleware/ratelimiter.go#L25
			Freq: 90,
			Per:  1 * time.Second,
		}
		duration := 20 * time.Second
		targeter := vegeta.NewStaticTargeter(vegeta.Target{
			Method: "GET",
			URL:    "https://localhost:65081/check/67",
		})
		attacker := vegeta.NewAttacker()

		var metrics vegeta.Metrics
		for res := range attacker.Attack(targeter, rate, duration, "rate_limit_test") {
			metrics.Add(res)
		}
		metrics.Close()

		fmt.Println("\n\t metrics:")
		fmt.Println(metrics)
		fmt.Println("\n.")

		expectedSuccesses := 1782
		attest.Approximately(t,
			// Actually, we would expect 1800 successes(20 *90) since the sending rate is 90/secs
			// which is below the ratelimit of 100/sec.
			// But ratelimiting is imprecise; https://github.com/komuw/ong/issues/235
			metrics.StatusCodes[fmt.Sprintf("%d", http.StatusOK)],
			expectedSuccesses,
			3,
		)

		attest.Subsequence(t,
			strings.Join(metrics.Errors, " "),
			http.StatusText(http.StatusTooManyRequests),
		)
	})
}
