package server

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/komuw/ong/log"

	"go.akshayshah.org/attest"
)

func TestPprofServer(t *testing.T) {
	t.Parallel()

	l := log.New(&bytes.Buffer{}, 500)(context.Background())

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		startPprofServer(l)

		// await for the server to start.
		port := 65060
		ping(t, uint16(port))

		uri := "/debug/pprof/heap"
		res, err := http.Get(fmt.Sprintf("http://localhost:%d%s", port, uri))
		attest.Ok(t, err)
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusOK)
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		startPprofServer(l)

		// await for the server to start.
		port := 65060
		ping(t, uint16(port))

		runhandler := func() {
			uri := "/debug/pprof/heap"
			res, err := http.Get(fmt.Sprintf("http://localhost:%d%s", port, uri))
			attest.Ok(t, err)
			defer res.Body.Close()

			attest.Equal(t, res.StatusCode, http.StatusOK)
		}

		wg := &sync.WaitGroup{}
		for rN := 0; rN <= 10; rN++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				runhandler()
			}()
		}
		wg.Wait()
	})
}
