// Package server provides HTTP server implementation.
// The server provided in here is opinionated and comes with good defaults.
package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	gowebErrors "github.com/komuw/goweb/errors"
	"github.com/komuw/goweb/log"

	"go.uber.org/automaxprocs/maxprocs"
	"golang.org/x/sys/unix" // syscall package is deprecated
)

type extendedHandler interface {
	Routes()
	GetLogger() log.Logger
	ServeHTTP(http.ResponseWriter, *http.Request)
}

// opts defines parameters for running an HTTP server.
type opts struct {
	port              string
	host              string
	network           string
	readHeaderTimeout time.Duration
	readTimeout       time.Duration
	writeTimeout      time.Duration
	handlerTimeout    time.Duration
	idleTimeout       time.Duration
}

// Equal compares two opts for equality.
// It was added for testing purposes.
func (rc opts) Equal(other opts) bool {
	return rc == other
}

// NewOpts returns a new opts.
func NewOpts(
	port string,
	host string,
	network string,
	readHeaderTimeout time.Duration,
	readTimeout time.Duration,
	writeTimeout time.Duration,
	handlerTimeout time.Duration,
	idleTimeout time.Duration,
) opts {
	return opts{
		port:              port,
		host:              host,
		network:           network,
		readHeaderTimeout: readHeaderTimeout,
		readTimeout:       readTimeout,
		writeTimeout:      writeTimeout,
		handlerTimeout:    handlerTimeout,
		idleTimeout:       idleTimeout,
	}
}

// WithOpts returns a new opts that has sensible defaults given port and host.
func WithOpts(port, host string) opts {
	// readHeaderTimeout < readTimeout < writeTimeout < handlerTimeout < idleTimeout
	// drainDuration = max(readHeaderTimeout , readTimeout , writeTimeout , handlerTimeout)

	readHeaderTimeout := 1 * time.Second
	readTimeout := readHeaderTimeout + (1 * time.Second)
	writeTimeout := readTimeout + (1 * time.Second)
	handlerTimeout := writeTimeout + (10 * time.Second)
	idleTimeout := handlerTimeout + (100 * time.Second)

	return NewOpts(
		port,
		host,
		"tcp",
		readHeaderTimeout,
		readTimeout,
		writeTimeout,
		handlerTimeout,
		idleTimeout,
	)
}

// DefaultOpts returns a new opts that has sensible defaults.
func DefaultOpts() opts {
	return WithOpts("8080", "127.0.0.1")
}

// Run listens on a network address and then calls Serve to handle requests on incoming connections.
// It sets up a server with the parameters provided by rc.
//
// The server shuts down cleanly after receiving any terminating signal.
func Run(eh extendedHandler, rc opts) error {
	setRlimit()
	_, _ = maxprocs.Set()

	eh.Routes()

	ctx, cancel := context.WithCancel(context.Background())
	logger := eh.GetLogger().WithCtx(ctx).WithImmediate()

	serverPort := fmt.Sprintf(":%s", rc.port)
	server := &http.Server{
		Addr: serverPort,

		// 1. https://blog.simon-frey.eu/go-as-in-golang-standard-net-http-config-will-break-your-production
		// 2. https://blog.cloudflare.com/exposing-go-on-the-internet/
		// 3. https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
		// 4. https://github.com/golang/go/issues/27375
		Handler:           http.TimeoutHandler(eh, rc.handlerTimeout, fmt.Sprintf("goweb: Handler timeout exceeded: %s", rc.handlerTimeout)),
		ReadHeaderTimeout: rc.readHeaderTimeout,
		ReadTimeout:       rc.readTimeout,
		WriteTimeout:      rc.writeTimeout,
		IdleTimeout:       rc.idleTimeout,
		ErrorLog:          logger.StdLogger(),
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}

	drainDur := drainDuration(rc)
	sigHandler(server, ctx, cancel, logger, drainDur)

	address := fmt.Sprintf("%s%s", rc.host, serverPort)

	err := serve(ctx, server, rc.network, address, logger)
	if !errors.Is(err, http.ErrServerClosed) {
		// The docs for http.server.Shutdown() says:
		//   When Shutdown is called, Serve/ListenAndServe/ListenAndServeTLS immediately return ErrServerClosed.
		//   Make sure the program doesn't exit and waits instead for Shutdown to return.

		return err // already wrapped in the `serve` func.
	}

	{
		// wait for server.Shutdown() to return.
		// cancel context incase drainDuration expires befure server.Shutdown() has completed.
		time.Sleep(drainDur)
		cancel()
	}

	return nil
}

func sigHandler(
	srv *http.Server,
	ctx context.Context,
	cancel context.CancelFunc,
	logger log.Logger,
	drainDur time.Duration,
) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, unix.SIGTERM, unix.SIGINT, unix.SIGQUIT, unix.SIGHUP)
	go func() {
		defer cancel()

		sigCaught := <-sigs
		logger.Info(log.F{
			"msg":              "server got shutdown signal",
			"signal":           fmt.Sprintf("%v", sigCaught),
			"shutdownDuration": drainDur.String(),
		})

		err := srv.Shutdown(ctx)
		if err != nil {
			logger.Error(err, log.F{
				"msg": "server shutdown error",
			})
		}
	}()
}

func serve(ctx context.Context, srv *http.Server, network, address string, logger log.Logger) error {
	cfg := &net.ListenConfig{Control: func(network, address string, conn syscall.RawConn) error {
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
	}}
	l, err := cfg.Listen(ctx, network, address)
	if err != nil {
		return gowebErrors.Wrap(err)
	}

	logger.Info(log.F{
		"msg": fmt.Sprintf("server listening at %s", address),
	})
	if errS := srv.Serve(l); errS != nil {
		return gowebErrors.Wrap(errS)
	}
	return nil
}

// drainDuration determines how long to wait for the server to shutdown after it has received a shutdown signal.
func drainDuration(rc opts) time.Duration {
	dur := 1 * time.Second

	if rc.handlerTimeout > dur {
		dur = rc.handlerTimeout
	}
	if rc.readHeaderTimeout > dur {
		dur = rc.readHeaderTimeout
	}
	if rc.readTimeout > dur {
		dur = rc.readTimeout
	}
	if rc.writeTimeout > dur {
		dur = rc.writeTimeout
	}

	// drainDuration should not take into account rc.idleTimeout
	// because server.Shutdown() already closes all idle connections.

	dur = dur + (10 * time.Second)

	return dur
}
