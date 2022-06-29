package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	gowebErrors "github.com/komuw/goweb/errors"
	"github.com/komuw/goweb/log"
)

func startPprofServer() {
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
	addr := fmt.Sprintf("localhost:%d", port)
	readHeader, read, write, idle := pprofTimeouts()
	pprofSrv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: readHeader,
		ReadTimeout:       read,
		WriteTimeout:      write,
		IdleTimeout:       idle,
		ErrorLog:          logger.StdLogger(),
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}

	go func() {
		logger.Info(log.F{
			"msg": fmt.Sprintf("pprof server listening at %s", pprofSrv.Addr),
		})
		errPprofSrv := pprofSrv.ListenAndServe()
		if errPprofSrv != nil {
			errPprofSrv = gowebErrors.Wrap(errPprofSrv)
			logger.Error(errPprofSrv, log.F{"msg": "unable to start pprof server"})
		}
	}()
}

func pprofTimeouts() (readHeader, read, write, idle time.Duration) {
	readHeader = 5 * time.Second
	read = readHeader + (20 * time.Second)
	write = 5 * time.Minute
	idle = 3 * time.Minute
	return readHeader, read, write, idle
}
