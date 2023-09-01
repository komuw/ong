package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/komuw/ong/config"
)

// TODO: tests.
// Add integration tests.

// pprofHandler is used to handle requests to the pprof endpoints.
// The endpoints are secured with Basic authentication. The username and password are [config.Opts.SecretKey]
func pprofHandler(o config.Opts) http.HandlerFunc {
	var (
		/*
			The pprof tool supports fetching profles by duration.
			eg; fetch cpu profile for the last 5mins(300sec):
				go tool pprof http://localhost:65079/debug/pprof/profile?seconds=300
			This may fail with an error like:
				http://localhost:65079/debug/pprof/profile?seconds=300: server response: 400 Bad Request - profile duration exceeds server's WriteTimeout
			So we need to be generous with our timeouts. Which is okay since pprof runs in a mux that is not exposed to the internet(localhost)
		*/
		readTimeout  = (o.ReadTimeout + 5*time.Minute)
		writeTimeout = (o.WriteTimeout + 30*time.Minute)
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
				e := fmt.Errorf("ong/server/pprof: cannot set SetReadDeadline(%s): %w", readTimeout, err)
				http.Error(w, e.Error(), http.StatusInternalServerError)
				return
			}

			if err := rc.SetWriteDeadline(now.Add(writeTimeout)); err != nil {
				e := fmt.Errorf("ong/server/pprof: cannot set SetWriteDeadline(%s): %w", writeTimeout, err)
				http.Error(w, e.Error(), http.StatusInternalServerError)
				return
			}
		}

		path := r.URL.Path
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
