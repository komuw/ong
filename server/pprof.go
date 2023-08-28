package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/komuw/ong/log"
)

/*
example usage:

	go tool pprof  http://localhost:65079/debug/pprof/heap
*/
func startPprofServer(l *slog.Logger, o Opts) {
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

	addr := fmt.Sprintf("127.0.0.1:%s", o.pprofPort)
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
		ErrorLog:          slog.NewLogLogger(l.Handler(), o.serverLogLevel),
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}

	go func() {
		cfg := listenerConfig()
		cl, err := cfg.Listen(ctx, "tcp", pprofSrv.Addr)
		if err != nil {
			l.Error("pprof server, unable to create listener", "error", err)
			return
		}

		slog.NewLogLogger(l.Handler(), log.LevelImmediate).
			Printf("pprof server listening at %s", pprofSrv.Addr)

		errPprofSrv := pprofSrv.Serve(cl)
		if errPprofSrv != nil {
			l.Error("unable to start pprof server", "error", errPprofSrv)
		}
	}()
}

func pprofTimeouts() (readHeader, read, write, idle time.Duration) {
	/*
		The pprof tool supports fetching profles by duration.
		eg; fetch cpu profile for the last 5mins(300sec):
			go tool pprof http://localhost:65079/debug/pprof/profile?seconds=300
		This may fail with an error like:
			http://localhost:65079/debug/pprof/profile?seconds=300: server response: 400 Bad Request - profile duration exceeds server's WriteTimeout
		So we need to be generous with our timeouts. Which is okay since pprof runs in a mux that is not exposed to the internet(localhost)
	*/
	readHeader = 7 * time.Second
	read = readHeader + (20 * time.Second)
	write = 20 * time.Minute
	idle = write + (3 * time.Minute)

	return readHeader, read, write, idle
}
