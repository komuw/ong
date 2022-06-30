package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	ongErrors "github.com/komuw/ong/errors"
	"github.com/komuw/ong/log"
)

/*
example usage:
  go tool pprof  http://localhost:6060/debug/pprof/heap
*/
func startPprofServer() {
	// This is taken from: https://github.com/golang/go/blob/go1.18.3/src/net/http/pprof/pprof.go#L80-L86
	//
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := log.New(
		context.Background(),
		os.Stdout, 1000, false).
		WithCtx(ctx).
		WithImmediate().
		WithFields(log.F{"pid": os.Getpid()})

	port := 6060
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	readHeader, read, write, idle := pprofTimeouts()
	pprofSrv := &http.Server{
		Addr: addr,
		// the pprof muxer is failing to work with `http.TimeoutHandler`
		// https://github.com/komuw/ong/issues/62
		Handler:           mux,
		ReadHeaderTimeout: readHeader,
		ReadTimeout:       read,
		WriteTimeout:      write,
		IdleTimeout:       idle,
		ErrorLog:          logger.StdLogger(),
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}

	go func() {
		cfg := listenerConfig()
		l, err := cfg.Listen(ctx, "tcp", pprofSrv.Addr)
		if err != nil {
			err = ongErrors.Wrap(err)
			logger.Error(err, log.F{"msg": "pprof server, unable to create listener"})
			return
		}

		logger.Info(log.F{
			"msg": fmt.Sprintf("pprof server listening at %s", pprofSrv.Addr),
		})
		errPprofSrv := pprofSrv.Serve(l)
		if errPprofSrv != nil {
			errPprofSrv = ongErrors.Wrap(errPprofSrv)
			logger.Error(errPprofSrv, log.F{"msg": "unable to start pprof server"})
		}
	}()
}

func pprofTimeouts() (readHeader, read, write, idle time.Duration) {
	/*
		The pprof tool supports fetching profles by duration.
		eg; fetch cpu profile for the last 5mins(300sec):
			go tool pprof http://localhost:6060/debug/pprof/profile?seconds=300
		This may fail with an error like:
			http://localhost:6060/debug/pprof/profile?seconds=300: server response: 400 Bad Request - profile duration exceeds server's WriteTimeout
		So we need to be generous with our timeouts. Which is okay since pprof runs in a mux that is not exposed to the internet(localhost)
	*/
	readHeader = 7 * time.Second
	read = readHeader + (20 * time.Second)
	write = 20 * time.Minute
	idle = write + (3 * time.Minute)

	return readHeader, read, write, idle
}
