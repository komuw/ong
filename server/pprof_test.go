package server

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/komuw/ong/internal/tst"
	"github.com/komuw/ong/log"

	"go.akshayshah.org/attest"
)

func TestPprofServer(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	l := log.New(context.Background(), buf, 500)
	port := 65079
	certFile, keyFile := createDevCertKey(l)
	o := Opts{
		serverLogLevel: defaultServerLogLevel,
		pprofPort:      fmt.Sprintf("%d", port),
		tls: tlsOpts{
			domain:   "localhost",
			certFile: certFile,
			keyFile:  keyFile,
		},
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		startPprofServer(o, l)

		// time.Sleep(4 * time.Second)
		// fmt.Println("log: ", buf.String())

		// await for the server to start.
		attest.Ok(t, tst.Ping(uint16(port)))

		uri := "/debug/pprof/heap"
		res, err := http.Get(fmt.Sprintf("https://localhost:%d%s", port, uri))
		attest.Ok(t, err)
		defer res.Body.Close()

		attest.Equal(t, res.StatusCode, http.StatusOK)
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		startPprofServer(o, l)

		// await for the server to start.
		attest.Ok(t, tst.Ping(uint16(port)))

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
