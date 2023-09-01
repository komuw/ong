// Package server provides HTTP server implementation.
// The server provided in here is opinionated and comes with good defaults.
package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/komuw/ong/automax"
	"github.com/komuw/ong/config"
	"github.com/komuw/ong/internal/acme"
	"github.com/komuw/ong/internal/finger"
	"github.com/komuw/ong/internal/octx"
	"github.com/komuw/ong/log"
	"github.com/komuw/ong/mux"

	"golang.org/x/sys/unix" // syscall package is deprecated
)

// Run creates a http server, starts the server on a network address and then calls Serve to handle requests on incoming connections.
//
// It sets up a server with the parameters provided by o.
// If the Opts supplied include a certificate and key, the server will accept https traffic and also automatically handle http->https redirect.
// Likewise, if the Opts include an acmeEmail address, the server will accept https traffic and automatically handle http->https redirect.
//
// The server shuts down cleanly after receiving any termination signal.
func Run(h http.Handler, o config.Opts) error {
	_ = automax.SetCpu()
	_ = automax.SetMem()

	{ // Add ACME route handler.
		if m, ok := h.(mux.Muxer); ok {
			// Support for acme certificate manager needs to be added in three places:
			// (a) In http middlewares.
			// (b) In http server.
			// (c) In http multiplexer.
			const acmeChallengeURI = "/.well-known/acme-challenge/:token"
			if err := m.Unwrap().AddRoute(
				mux.NewRoute(
					acmeChallengeURI,
					mux.MethodAll,
					acme.Handler(m),
				),
			); err != nil {
				return fmt.Errorf("ong/server: unable to add ACME handler: %w", err)
			}
		}
	}

	{ // Add pprof route handler.
		// TODO:
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tlsConf, errTc := getTlsConfig(o)
	if errTc != nil {
		return errTc
	}
	server := &http.Server{
		Addr:      o.ServerPort,
		TLSConfig: tlsConf,

		// 1. https://blog.simon-frey.eu/go-as-in-golang-standard-net-http-config-will-break-your-production
		// 2. https://blog.cloudflare.com/exposing-go-on-the-internet/
		// 3. https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
		// 4. https://github.com/golang/go/issues/27375
		Handler: http.MaxBytesHandler(
			h,
			int64(o.MaxBodyBytes), // limit in bytes.
		),
		// http.TimeoutHandler does not implement [http.ResponseController] so we no longer use it.

		ReadHeaderTimeout: o.ReadHeaderTimeout,
		ReadTimeout:       o.ReadTimeout,
		WriteTimeout:      o.WriteTimeout,
		IdleTimeout:       o.IdleTimeout,
		ErrorLog:          slog.NewLogLogger(o.Logger.Handler(), o.ServerLogLevel),
		BaseContext:       func(net.Listener) context.Context { return ctx },
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			tConn, ok := c.(*tls.Conn)
			if !ok {
				return ctx
			}

			conn, ok := tConn.NetConn().(*fingerConn)
			if !ok {
				return ctx
			}

			fPrint := conn.fingerprint.Load()
			if fPrint == nil {
				fPrint = &finger.Print{}
				conn.fingerprint.CompareAndSwap(nil, fPrint)
			}
			return context.WithValue(ctx, octx.FingerPrintCtxKey, fPrint)
		},
	}

	sigHandler(server, ctx, cancel, o.Logger, o.DrainTimeout)

	err := serve(ctx, server, o)
	if !errors.Is(err, http.ErrServerClosed) {
		// The docs for http.server.Shutdown() says:
		//   When Shutdown is called, Serve/ListenAndServe/ListenAndServeTLS immediately return ErrServerClosed.
		//   Make sure the program doesn't exit and waits instead for Shutdown to return.
		//
		return err // already wrapped in the `serve` func.
	}

	return nil
}

func sigHandler(
	srv *http.Server,
	ctx context.Context,
	cancel context.CancelFunc,
	logger *slog.Logger,
	drainDur time.Duration,
) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, unix.SIGTERM, unix.SIGINT, unix.SIGQUIT, unix.SIGHUP)
	go func() {
		defer cancel()

		sigCaught := <-sigs

		sl := slog.NewLogLogger(logger.Handler(), log.LevelImmediate)
		sl.Println("server got shutdown signal.",
			"signal =", fmt.Sprintf("%v.", sigCaught),
			"shutdownDuration =", drainDur.String(),
		)

		{
			// If your app is running in kubernetes(k8s), if a pod is deleted;
			// (a) it gets deleted from service endpoints.
			// (b) it receives a SIGTERM from kubelet.
			// (a) & (b) are done concurrently. Thus (b) can occur before (a);
			// if that is the case; your app will shutdown while k8s is still sending traffic to it.
			// This sleep here, minimizes the duration of that race condition.
			// - https://twitter.com/ProgrammerDude/status/1660238268863066114
			// - https://twitter.com/thockin/status/1560398974929973248
			time.Sleep(drainDur)
		}

		err := srv.Shutdown(ctx)
		if err != nil {
			logger.Error("server shutdown error", "error", err)
		}
	}()
}

func serve(ctx context.Context, srv *http.Server, o config.Opts) error {
	{
		// HTTP(non-tls) LISTERNER:
		redirectSrv := &http.Server{
			Addr:              fmt.Sprintf("%s%s", o.Host, o.HttpPort),
			Handler:           srv.Handler,
			ReadHeaderTimeout: o.ReadHeaderTimeout,
			ReadTimeout:       o.ReadTimeout,
			WriteTimeout:      o.WriteTimeout,
			IdleTimeout:       o.IdleTimeout,
			ErrorLog:          slog.NewLogLogger(o.Logger.Handler(), o.ServerLogLevel),
			BaseContext:       func(net.Listener) context.Context { return ctx },
		}
		go func() {
			redirectSrvCfg := listenerConfig()
			redirectSrvListener, errL := redirectSrvCfg.Listen(ctx, "tcp", redirectSrv.Addr)
			if errL != nil {
				o.Logger.Error("redirect server, unable to create listener", "error", errL)
				return
			}

			slog.NewLogLogger(o.Logger.Handler(), log.LevelImmediate).
				Printf("redirect server listening at %s", redirectSrv.Addr)
			errRedirectSrv := redirectSrv.Serve(redirectSrvListener)
			if errRedirectSrv != nil {
				o.Logger.Error("unable to start redirect server", "error", errRedirectSrv)
			}
		}()
	}

	{
		// HTTPS(tls) LISTERNER:
		cfg := listenerConfig()
		cl, err := cfg.Listen(ctx, o.Network, o.ServerAddress)
		if err != nil {
			return err
		}

		l := &fingerListener{cl}

		slog.NewLogLogger(o.Logger.Handler(), log.LevelImmediate).Printf("https server listening at %s", o.ServerAddress)
		if errS := srv.ServeTLS(
			l,
			// use empty cert & key. they will be picked from `srv.TLSConfig`
			"",
			"",
		); errS != nil {
			return errS
		}
	}

	return nil
}

// listenerConfig creates a net listener config that reuses address and port.
// This is essential in order to be able to carry out zero-downtime deploys.
func listenerConfig() *net.ListenConfig {
	return &net.ListenConfig{
		Control: func(network, address string, conn syscall.RawConn) error {
			return conn.Control(func(descriptor uintptr) {
				_ = unix.SetsockoptInt(
					int(descriptor),
					unix.SOL_SOCKET,
					// go vet will complain if we used syscall.SO_REUSEPORT, even though it would work.
					// this is because Go considers syscall pkg to be frozen. The same goes for syscall.SetsockoptInt
					// so we use x/sys/unix
					// see: https://github.com/golang/go/issues/26771
					unix.SO_REUSEPORT,
					1,
				)
				_ = unix.SetsockoptInt(
					int(descriptor),
					unix.SOL_SOCKET,
					unix.SO_REUSEADDR,
					1,
				)
			})
		},
	}
}
