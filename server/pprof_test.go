package server

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/akshayjshah/attest"
)

func TestPprofServer(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		startPprofServer()

		// await for the server to start.
		time.Sleep((1 * time.Second))

		uri := "/debug/pprof/heap"
		port := 6060
		res, err := http.Get(fmt.Sprintf("http://localhost:%d%s", port, uri))
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
	})
}
