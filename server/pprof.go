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

// TODO: remove this file???

/*
example usage:

	go tool pprof  http://localhost:65079/debug/pprof/heap
*/
func startPprofServer(logger *slog.Logger, o Opts) {
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
		ErrorLog:          slog.NewLogLogger(logger.Handler(), o.serverLogLevel),
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}

	go func() {
		cfg := listenerConfig()
		l, err := cfg.Listen(ctx, "tcp", pprofSrv.Addr)
		if err != nil {
			logger.Error("pprof server, unable to create listener", "error", err)
			return
		}

		slog.NewLogLogger(logger.Handler(), log.LevelImmediate).
			Printf("pprof server listening at %s", pprofSrv.Addr)

		errPprofSrv := pprofSrv.Serve(l)
		if errPprofSrv != nil {
			logger.Error("unable to start pprof server", "error", errPprofSrv)
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

////////// TODO:

// pprofHandler is used to handle requests to the pprof endpoints.
func pprofHandler() http.HandlerFunc {
	const (
		/*
			The pprof tool supports fetching profles by duration.
			eg; fetch cpu profile for the last 5mins(300sec):
				go tool pprof http://localhost:65079/debug/pprof/profile?seconds=300
			This may fail with an error like:
				http://localhost:65079/debug/pprof/profile?seconds=300: server response: 400 Bad Request - profile duration exceeds server's WriteTimeout
			So we need to be generous with our timeouts. Which is okay since pprof runs in a mux that is not exposed to the internet(localhost)
		*/
		readTimeout  = 5 * time.Minute
		writeTimeout = 30 * time.Minute
	)

	// Unfortunately the pprof endpoints do not interact well with [http.ResponseController]
	// We need to inject a fake server with a large WriteTimeout in order for things to work out well.
	//
	// See: https://github.com/golang/go/issues/62358
	fakeSrv := &http.Server{
		WriteTimeout: writeTimeout + (3 * time.Second),
	}

	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		{
			orginalSrv, ok := ctx.Value(http.ServerContextKey).(*http.Server)
			defer func() {
				if ok {
					// At the end, set the server to its original value.
					ctx = context.WithValue(ctx, http.ServerContextKey, orginalSrv)
					r = r.WithContext(ctx)
				}
			}()

			// Use a fake server with a long timeout.
			ctx = context.WithValue(ctx, http.ServerContextKey, fakeSrv)
			r = r.WithContext(ctx)
		}

		{
			now := time.Now()
			rc := http.NewResponseController(w)

			if err := rc.SetReadDeadline(now.Add(readTimeout)); err != nil {
				e := fmt.Errorf("ong/server: cannot set SetReadDeadline(%s): %w", readTimeout, err)
				http.Error(w, e.Error(), http.StatusInternalServerError)
				return
			}

			if err := rc.SetWriteDeadline(now.Add(writeTimeout)); err != nil {
				e := fmt.Errorf("ong/server: cannot set SetWriteDeadline(%s): %w", writeTimeout, err)
				http.Error(w, e.Error(), http.StatusInternalServerError)
				return
			}
		}

		path := r.URL.Path
		fmt.Println("\n\t pprofTT called. path: ", path)

		switch path {
		default:
			pprof.Index(w, r)
			return
		case "/debug/pprof/cmdline":
			pprof.Cmdline(w, r)
			return
		case "/debug/pprof/profile":
			pprof.Profile(w, r)
			return
		case "/debug/pprof/symbol":
			pprof.Symbol(w, r)
			return
		case "/debug/pprof/trace":
			pprof.Trace(w, r)
			return
		}
	}
}
