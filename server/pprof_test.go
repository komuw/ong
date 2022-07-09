package server

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/akshayjshah/attest"
	"github.com/komuw/ong/log"
)

func TestPprofServer(t *testing.T) {
	t.Parallel()

	l := log.New(context.Background(), &bytes.Buffer{}, 500)

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		startPprofServer(l)

		// await for the server to start.
		time.Sleep(1 * time.Second)

		uri := "/debug/pprof/heap"
		port := 6060
		res, err := http.Get(fmt.Sprintf("http://localhost:%d%s", port, uri))
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		startPprofServer(l)

		// await for the server to start.
		time.Sleep(1 * time.Second)

		runhandler := func() {
			uri := "/debug/pprof/heap"
			port := 6060
			res, err := http.Get(fmt.Sprintf("http://localhost:%d%s", port, uri))
			attest.Ok(t, err)

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
