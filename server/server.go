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

func Run(eh extendedHandler) error {
	setRlimit()
	maxprocs.Set()

	eh.Routes()

	ctx, cancel := context.WithCancel(context.Background())
	serverPort := ":8080"
	network := "tcp"
	address := fmt.Sprintf("127.0.0.1%s", serverPort)
	server := &http.Server{
		Addr: serverPort,

		// 1. https://blog.simon-frey.eu/go-as-in-golang-standard-net-http-config-will-break-your-production
		// 2. https://blog.cloudflare.com/exposing-go-on-the-internet/
		// 3. https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
		// 4. https://github.com/golang/go/issues/27375
		Handler:           http.TimeoutHandler(eh, 10*time.Second, "Custom Server timeout"),
		ReadHeaderTimeout: 1 * time.Second,
		ReadTimeout:       1 * time.Second,
		WriteTimeout:      1 * time.Second,
		IdleTimeout:       120 * time.Second,

		BaseContext: func(net.Listener) context.Context { return ctx },
	}

	sigHandler(server, ctx, cancel, eh.GetLogger())

	eh.GetLogger().Printf("server listening at %s", address)
	return serve(server, network, address, ctx)
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
