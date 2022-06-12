package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/automaxprocs/maxprocs"
	"golang.org/x/sys/unix" // syscall package is deprecated
)

type extendedHandler interface {
	Routes()
	GetLogger() *log.Logger
	ServeHTTP(http.ResponseWriter, *http.Request)
}

type runContext struct {
	port              string
	network           string
	host              string
	handlerTimeout    time.Duration
	readHeaderTimeout time.Duration
	readTimeout       time.Duration
	writeTimeout      time.Duration
	idleTimeout       time.Duration
}

func NewRunContext(
	port string,
	network string,
	host string,
	handlerTimeout time.Duration,
	readHeaderTimeout time.Duration,
	readTimeout time.Duration,
	writeTimeout time.Duration,
	idleTimeout time.Duration,
) runContext {
	return runContext{
		port:              port,
		network:           network,
		host:              host,
		handlerTimeout:    handlerTimeout,
		readHeaderTimeout: readHeaderTimeout,
		readTimeout:       readTimeout,
		writeTimeout:      writeTimeout,
		idleTimeout:       idleTimeout,
	}
}

func DefaultRunContext() runContext {
	return runContext{
		port:              "8080",
		network:           "tcp",
		host:              "127.0.0.1",
		handlerTimeout:    10 * time.Second,
		readHeaderTimeout: 1 * time.Second,
		readTimeout:       1 * time.Second,
		writeTimeout:      1 * time.Second,
		idleTimeout:       120 * time.Second,
	}
}

func Run(eh extendedHandler, rc runContext) error {
	setRlimit()
	maxprocs.Set()

	eh.Routes()

	ctx, cancel := context.WithCancel(context.Background())

	serverPort := fmt.Sprintf(":%s", rc.port)
	server := &http.Server{
		Addr: serverPort,

		// 1. https://blog.simon-frey.eu/go-as-in-golang-standard-net-http-config-will-break-your-production
		// 2. https://blog.cloudflare.com/exposing-go-on-the-internet/
		// 3. https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
		// 4. https://github.com/golang/go/issues/27375
		Handler:           http.TimeoutHandler(eh, rc.handlerTimeout, "Custom Server timeout"),
		ReadHeaderTimeout: rc.readHeaderTimeout,
		ReadTimeout:       rc.readTimeout,
		WriteTimeout:      rc.writeTimeout,
		IdleTimeout:       rc.idleTimeout,
		ErrorLog:          eh.GetLogger(),
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}

	sigHandler(server, ctx, cancel, eh.GetLogger())

	address := fmt.Sprintf("%s%s", rc.host, serverPort)
	eh.GetLogger().Printf("server listening at %s", address)
	return serve(server, rc.network, address, ctx)
}

func sigHandler(srv *http.Server, ctx context.Context, cancel context.CancelFunc, logger *log.Logger) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, unix.SIGTERM, unix.SIGINT, unix.SIGQUIT, unix.SIGHUP)
	go func() {
		defer cancel()

		sigCaught := <-sigs
		logger.Println("server got shutdown signal: ", sigCaught)

		err := srv.Shutdown(ctx)
		if err != nil {
			logger.Println("server shutdown error: ", err)
		}
	}()
}

func serve(srv *http.Server, network, address string, ctx context.Context) error {
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
		return err
	}

	return srv.Serve(l)
}
