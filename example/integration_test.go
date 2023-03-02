package main_test

import (
	"fmt"
	"testing"
	"time"

	vegeta "github.com/tsenart/vegeta/v12/lib"
)

// go:build integration

func TestIntegration(t *testing.T) {
	t.Parallel()

	t.Run("https_redirection", func(t *testing.T) {
		t.Parallel()

		c := &http.Client{}
		c.Get("http://127.0.0.1:65080/health")

		// 		status_code=$(curl -k --write-out '%{http_code}' --silent --output /dev/null "http://127.0.0.1:65080/health")
		//   if [[ "$status_code" -ne 308 ]] ; then
		//     echo "expected 127.0.0.1 to be redirected to localhost"
		//     exit 61;
		//   fi

		//   rate := vegeta.Rate{Freq: 1, Per: time.Second}
		// 	duration := 1 * time.Second
		// 	targeter := vegeta.NewStaticTargeter(vegeta.Target{
		// 		Method: "GET",
		// 		URL:    "http://localhost:9100/",
		// 	})
		// 	attacker := vegeta.NewAttacker()

		// 	var metrics vegeta.Metrics
		// 	for res := range attacker.Attack(targeter, rate, duration, "Big Bang!") {
		// 		metrics.Add(res)
		// 	}
		// 	metrics.Close()

		// 	fmt.Printf("99th percentile: %s\n", metrics.Latencies.P99)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)
	})

}
