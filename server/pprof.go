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
func startPprofServer(o Opts, l *slog.Logger) {
	// This is taken from: https://github.com/golang/go/blob/go1.21.0/src/net/http/pprof/pprof.go#L93-L99
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

	tlsConf, errTc := getTlsConfig(o, l)
	if errTc != nil {
		l.Error("pprof server, unable to get TLS config", "error", errTc)
		return
	}

	pprofSrv := &http.Server{
		Addr:      addr,
		TLSConfig: tlsConf,

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
		// cfg := listenerConfig()
		// cl, err := cfg.Listen(ctx, "tcp", pprofSrv.Addr)

		cl, err := net.Listen("tcp", pprofSrv.Addr)
		if err != nil {
			l.Error("pprof server, unable to create listener", "error", err)
			return
		}

		slog.NewLogLogger(l.Handler(), log.LevelImmediate).
			Printf("pprof https server listening at %s", pprofSrv.Addr)

		errPprofSrv := pprofSrv.ServeTLS(
			cl,
			// use empty cert & key. they will be picked from `pprofSrv.TLSConfig`
			"",
			"",
		)
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
	readHeader = 13 * time.Second
	read = readHeader + (30 * time.Second)
	write = 30 * time.Minute
	idle = write + (3 * time.Minute)

	return readHeader, read, write, idle
}
