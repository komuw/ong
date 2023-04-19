// Package server provides HTTP server implementation.
// The server provided in here is opinionated and comes with good defaults.
package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/komuw/ong/automax"
	"github.com/komuw/ong/internal/finger"
	"github.com/komuw/ong/internal/octx"
	"github.com/komuw/ong/log"

	"golang.org/x/exp/slog"
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

// Opts are the various parameters(optionals) that can be used to configure a HTTP server.
//
// Use either [NewOpts], [DevOpts], [CertOpts] or [LetsEncryptOpts] to get a valid Opts.
type Opts struct {
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

// Equal compares two Opts for equality.
// It was added for testing purposes.
func (o Opts) Equal(other Opts) bool {
	return o == other
}

// NewOpts returns a new Opts.
//
// port is the port at which the server should listen on.
//
// readHeaderTimeout is the amount of time a server will be allowed to read request headers.
// readTimeout is the maximum duration a server will use for reading the entire request, including the body.
// writeTimeout is the maximum duration before a server times out writes of the response.
// handlerTimeout is the maximum duration that handlers on the server will serve a request before timing out.
// idleTimeout is the maximum amount of time to wait for the next request when keep-alives are enabled.
//
// certFile is a path to a tls certificate.
// keyFile is a path to a tls key.
//
// email is the e-address that will be used if/when procuring certificates from [letsencrypt].
// domain is the domain name of your website; it can be an exact domain, subdomain or wildcard.
//
// If certFile is a non-empty string, this will enable tls using certificates found on disk.
// If email is a non-empty string, this will enable tls using certificates procured from [letsencrypt].
//
// [letsencrypt]: https://letsencrypt.org/
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
) Opts {
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

	return Opts{
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

// DevOpts returns a new Opts that has sensible defaults for tls, especially for dev environments.
// It also automatically creates the dev certifiates/key.
func DevOpts(l *slog.Logger) Opts {
	certFile, keyFile := createDevCertKey(l)

	return withOpts(65081, certFile, keyFile, "", "localhost")
}

// CertOpts returns a new Opts that has sensible defaults given certFile & keyFile.
func CertOpts(certFile, keyFile, domain string) Opts {
	return withOpts(443, certFile, keyFile, "", domain)
}

// LetsEncryptOpts returns a new Opts that procures certificates from Letsencrypt.
func LetsEncryptOpts(email, domain string) Opts {
	return withOpts(443, "", "", email, domain)
}

// withOpts returns a new Opts that has sensible defaults given port.
func withOpts(port uint16, certFile, keyFile, email, domain string) Opts {
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

// Run creates a http server, starts the server on a network address and then calls Serve to handle requests on incoming connections.
//
// It sets up a server with the parameters provided by o.
// If the Opts supplied include a certificate and key, the server will accept https traffic and also automatically handle http->https redirect.
// Likewise, if the Opts include an email address, the server will accept https traffic and automatically handle http->https redirect.
//
// The server shuts down cleanly after receiving any termination signal.
func Run(h http.Handler, o Opts, l *slog.Logger) error {
	_ = automax.SetCpu()
	_ = automax.SetMem()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
		ErrorLog:          slog.NewLogLogger(l.Handler(), slog.LevelDebug),
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

	drainDur := drainDuration(o)
	sigHandler(server, ctx, cancel, l, drainDur)

	{
		startPprofServer(l)
	}

	err := serve(ctx, server, o, l)
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

		err := srv.Shutdown(ctx)
		if err != nil {
			logger.Error("server shutdown error", err)
		}
	}()
}

func serve(ctx context.Context, srv *http.Server, o Opts, logger *slog.Logger) error {
	{
		// HTTP(non-tls) LISTERNER:
		redirectSrv := &http.Server{
			Addr:              fmt.Sprintf("%s%s", o.host, o.httpPort),
			Handler:           srv.Handler,
			ReadHeaderTimeout: o.readHeaderTimeout,
			ReadTimeout:       o.readTimeout,
			WriteTimeout:      o.writeTimeout,
			IdleTimeout:       o.idleTimeout,
			ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelDebug),
			BaseContext:       func(net.Listener) context.Context { return ctx },
		}
		go func() {
			redirectSrvCfg := listenerConfig()
			redirectSrvListener, errL := redirectSrvCfg.Listen(ctx, "tcp", redirectSrv.Addr)
			if errL != nil {
				logger.Error("redirect server, unable to create listener", errL)
				return
			}

			slog.NewLogLogger(logger.Handler(), log.LevelImmediate).
				Printf("redirect server listening at %s", redirectSrv.Addr)
			errRedirectSrv := redirectSrv.Serve(redirectSrvListener)
			if errRedirectSrv != nil {
				logger.Error("unable to start redirect server", errRedirectSrv)
			}
		}()
	}

	{
		// HTTPS(tls) LISTERNER:
		cfg := listenerConfig()
		cl, err := cfg.Listen(ctx, o.network, o.serverAddress)
		if err != nil {
			return err
		}

		l := &fingerListener{cl}

		slog.NewLogLogger(logger.Handler(), log.LevelImmediate).Printf("https server listening at %s", o.serverAddress)
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
func drainDuration(o Opts) time.Duration {
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
