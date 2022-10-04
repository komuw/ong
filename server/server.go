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

	"github.com/komuw/ong/automax"
	"github.com/komuw/ong/log"

	"golang.org/x/sys/unix" // syscall package is deprecated
)

type tlsOpts struct {
	// if certFile is present, tls will be served from certificates on disk.
	certFile string
	keyFile  string
	// if email is present, tls will be served from letsencrypt certifiates.
	email string
	// domain can be a wildcard.
	// However, the certificate issued will NOT be wildcard certs; since letsencrypt only issues wildcard certs via DNS-01 challenge
	// Instead, we'll get a certifiate per subdomain.
	// see; https://letsencrypt.org/docs/faq/#does-let-s-encrypt-issue-wildcard-certificates
	domain string
}

// opts defines parameters for running an HTTP server.
type opts struct {
	port              uint16 // tcp port is a 16bit unsigned integer.
	readHeaderTimeout time.Duration
	readTimeout       time.Duration
	writeTimeout      time.Duration
	handlerTimeout    time.Duration
	idleTimeout       time.Duration
	tls               tlsOpts
	// the following ones are created automatically
	host          string
	serverPort    string
	serverAddress string
	network       string
	httpPort      string
}

// Equal compares two opts for equality.
// It was added for testing purposes.
func (o opts) Equal(other opts) bool {
	return o == other
}

// NewOpts returns a new opts.
//
// If certFile is a non-empty string, this will enable tls from certificates found on disk.
// If email is a non-empty string, this will enable tls from certificates procured from letsencrypt.
// domain can be an exact domain, subdomain or wildcard.
func NewOpts(
	port uint16,
	readHeaderTimeout time.Duration,
	readTimeout time.Duration,
	writeTimeout time.Duration,
	handlerTimeout time.Duration,
	idleTimeout time.Duration,
	certFile string,
	keyFile string,
	email string, // if present, tls will be served from letsencrypt certifiates.
	domain string,
) opts {
	serverPort := fmt.Sprintf(":%d", port)
	host := "127.0.0.1"
	if port == 80 || port == 443 {
		// bind to both tcp4 and tcp6
		// https://github.com/golang/go/issues/48723
		host = "0.0.0.0"
	}
	serverAddress := fmt.Sprintf("%s%s", host, serverPort)

	httpPort := uint16(80)
	if port != 443 {
		httpPort = port - 1
	}

	return opts{
		port:              port,
		readHeaderTimeout: readHeaderTimeout,
		readTimeout:       readTimeout,
		writeTimeout:      writeTimeout,
		handlerTimeout:    handlerTimeout,
		idleTimeout:       idleTimeout,
		tls: tlsOpts{
			certFile: certFile,
			keyFile:  keyFile,
			email:    email,
			domain:   domain,
		},
		// this ones are created automatically
		host:          host,
		serverPort:    serverPort,
		serverAddress: serverAddress,
		network:       "tcp",
		httpPort:      fmt.Sprintf(":%d", httpPort),
	}
}

// DevOpts returns a new opts that has sensible defaults for tls, especially for dev environments.
// It also automatically creates the dev certifiates/key by internally calling [CreateDevCertKey]
func DevOpts() opts {
	if os.Getenv("ONG_RUNNING_IN_TESTS") == "" {
		// This means we are not in CI. Thus, create dev certificates.
		//
		// This function call fails in CI with `permission denied`
		// since it is trying to create certificates in filesystem.
		_, _ = CreateDevCertKey()
	}
	certFile, keyFile := certKeyPaths()
	return withOpts(65081, certFile, keyFile, "", "localhost")
}

// CertOpts returns a new opts that has sensible defaults given certFile & keyFile.
func CertOpts(certFile, keyFile, domain string) opts {
	return withOpts(443, certFile, keyFile, "", domain)
}

// LetsEncryptOpts returns a new opts that procures certificates from Letsencrypt.
func LetsEncryptOpts(email, domain string) opts {
	return withOpts(443, "", "", email, domain)
}

// withOpts returns a new opts that has sensible defaults given port.
func withOpts(port uint16, certFile, keyFile, email, domain string) opts {
	// readHeaderTimeout < readTimeout < writeTimeout < handlerTimeout < idleTimeout
	// drainDuration = max(readHeaderTimeout , readTimeout , writeTimeout , handlerTimeout)

	readHeaderTimeout := 1 * time.Second
	readTimeout := readHeaderTimeout + (1 * time.Second)
	writeTimeout := readTimeout + (1 * time.Second)
	handlerTimeout := writeTimeout + (10 * time.Second)
	idleTimeout := handlerTimeout + (100 * time.Second)

	return NewOpts(
		port,
		readHeaderTimeout,
		readTimeout,
		writeTimeout,
		handlerTimeout,
		idleTimeout,
		certFile,
		keyFile,
		email,
		domain,
	)
}

// Run listens on a network address and then calls Serve to handle requests on incoming connections.
//
// It sets up a server with the parameters provided by o.
// If the opts supplied include a certificate and key, the server will accept https traffic and also automatically handle http->https redirect.
//
// The server shuts down cleanly after receiving any terminating signal.
func Run(h http.Handler, o opts, l log.Logger) error {
	_ = automax.SetCpu()
	_ = automax.SetMem()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := l.WithCtx(ctx).WithImmediate().WithFields(log.F{"pid": os.Getpid()})

	tlsConf, errTc := getTlsConfig(o)
	if errTc != nil {
		return errTc
	}
	server := &http.Server{
		Addr:      o.serverPort,
		TLSConfig: tlsConf,

		// 1. https://blog.simon-frey.eu/go-as-in-golang-standard-net-http-config-will-break-your-production
		// 2. https://blog.cloudflare.com/exposing-go-on-the-internet/
		// 3. https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
		// 4. https://github.com/golang/go/issues/27375
		Handler: http.TimeoutHandler(
			h,
			o.handlerTimeout,
			fmt.Sprintf("ong: Handler timeout exceeded: %s", o.handlerTimeout),
		),
		ReadHeaderTimeout: o.readHeaderTimeout,
		ReadTimeout:       o.readTimeout,
		WriteTimeout:      o.writeTimeout,
		IdleTimeout:       o.idleTimeout,
		ErrorLog:          logger.StdLogger(),
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}

	drainDur := drainDuration(o)
	sigHandler(server, ctx, cancel, logger, drainDur)

	{
		startPprofServer(logger)
	}

	err := serve(ctx, server, o, logger)
	if !errors.Is(err, http.ErrServerClosed) {
		// The docs for http.server.Shutdown() says:
		//   When Shutdown is called, Serve/ListenAndServe/ListenAndServeTLS immediately return ErrServerClosed.
		//   Make sure the program doesn't exit and waits instead for Shutdown to return.
		//
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

func serve(ctx context.Context, srv *http.Server, o opts, logger log.Logger) error {
	{
		// HTTP(non-tls) LISTERNER:
		redirectSrv := &http.Server{
			Addr:              fmt.Sprintf("%s%s", o.host, o.httpPort),
			Handler:           srv.Handler,
			ReadHeaderTimeout: o.readHeaderTimeout,
			ReadTimeout:       o.readTimeout,
			WriteTimeout:      o.writeTimeout,
			IdleTimeout:       o.idleTimeout,
			ErrorLog:          logger.StdLogger(),
			BaseContext:       func(net.Listener) context.Context { return ctx },
		}
		go func() {
			redirectSrvCfg := listenerConfig()
			redirectSrvListener, errL := redirectSrvCfg.Listen(ctx, "tcp", redirectSrv.Addr)
			if errL != nil {
				logger.Error(errL, log.F{"msg": "redirect server, unable to create listener"})
				return
			}

			logger.Info(log.F{
				"msg": fmt.Sprintf("redirect server listening at %s", redirectSrv.Addr),
			})
			errRedirectSrv := redirectSrv.Serve(redirectSrvListener)
			if errRedirectSrv != nil {
				logger.Error(errRedirectSrv, log.F{"msg": "unable to start redirect server"})
			}
		}()
	}

	{
		// HTTPS(tls) LISTERNER:
		cfg := listenerConfig()
		l, err := cfg.Listen(ctx, o.network, o.serverAddress)
		if err != nil {
			return err
		}
		logger.Info(log.F{
			"msg": fmt.Sprintf("https server listening at %s", o.serverAddress),
		})
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

// drainDuration determines how long to wait for the server to shutdown after it has received a shutdown signal.
func drainDuration(o opts) time.Duration {
	dur := 1 * time.Second

	if o.handlerTimeout > dur {
		dur = o.handlerTimeout
	}
	if o.readHeaderTimeout > dur {
		dur = o.readHeaderTimeout
	}
	if o.readTimeout > dur {
		dur = o.readTimeout
	}
	if o.writeTimeout > dur {
		dur = o.writeTimeout
	}

	// drainDuration should not take into account o.idleTimeout
	// because server.Shutdown() already closes all idle connections.

	dur = dur + (10 * time.Second)

	return dur
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
