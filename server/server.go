// Package server provides HTTP server implementation.
// The server provided in here is opinionated and comes with good defaults.
package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	ongErrors "github.com/komuw/ong/errors"
	"github.com/komuw/ong/log"
	"github.com/komuw/ong/middleware"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/automaxprocs/maxprocs"
	"golang.org/x/sys/unix" // syscall package is deprecated
)

// extendedHandler is a http.Handler
type extendedHandler interface {
	GetLogger() log.Logger
	ServeHTTP(http.ResponseWriter, *http.Request)
}

// opts defines parameters for running an HTTP server.
type opts struct {
	port              uint16 // tcp port is a 16bit unsigned integer.
	host              string
	readHeaderTimeout time.Duration
	readTimeout       time.Duration
	writeTimeout      time.Duration
	handlerTimeout    time.Duration
	idleTimeout       time.Duration
	certFile          string
	keyFile           string

	// this ones are created automatically
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
func NewOpts(
	port uint16,
	host string,
	readHeaderTimeout time.Duration,
	readTimeout time.Duration,
	writeTimeout time.Duration,
	handlerTimeout time.Duration,
	idleTimeout time.Duration,
	certFile string,
	keyFile string,
) opts {
	serverPort := fmt.Sprintf(":%d", port)
	serverAddress := fmt.Sprintf("%s%s", host, serverPort)

	httpPort := port
	isTls := certFile != ""
	if isTls {
		if port == 443 {
			httpPort = 80
		} else {
			httpPort = port - 1
		}
	}

	return opts{
		port:              port,
		host:              host,
		readHeaderTimeout: readHeaderTimeout,
		readTimeout:       readTimeout,
		writeTimeout:      writeTimeout,
		handlerTimeout:    handlerTimeout,
		idleTimeout:       idleTimeout,
		certFile:          certFile,
		keyFile:           keyFile,
		// this ones are created automatically
		serverPort:    serverPort,
		serverAddress: serverAddress,
		network:       "tcp",
		httpPort:      fmt.Sprintf(":%d", httpPort),
	}
}

// WithOpts returns a new opts that has sensible defaults given port and host.
func WithOpts(port uint16, host string) opts {
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
		readHeaderTimeout,
		readTimeout,
		writeTimeout,
		handlerTimeout,
		idleTimeout,
		"",
		"",
	)
}

// WithTlsOpts returns a new opts that has sensible defaults given host, certFile & keyFile.
func WithTlsOpts(host, certFile, keyFile string) opts {
	return withTlsOpts(443, host, certFile, keyFile)
}

func withTlsOpts(port uint16, host, certFile, keyFile string) opts {
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
		readHeaderTimeout,
		readTimeout,
		writeTimeout,
		handlerTimeout,
		idleTimeout,
		certFile,
		keyFile,
	)
}

// DefaultOpts returns a new opts that has sensible defaults.
func DefaultOpts() opts {
	return WithOpts(8080, "127.0.0.1")
}

func DefaultTlsOpts() opts {
	certFile, keyFile := certKeyPaths()
	return withTlsOpts(8081, "127.0.0.1", certFile, keyFile)
}

// type cert struct {
// 	c      *tls.Certificate
// 	expiry time.Time
// }

// func newCert(certFile, keyFile string) (*cert, error) {
// 	c, err := tls.LoadX509KeyPair(certFile, keyFile)
// 	if err != nil {
// 		return nil, err
// 	}
// 	leaf, err := x509.ParseCertificate(c.Certificate[0])
// 	if err != nil {
// 		return nil, err
// 	}
// 	c.Leaf = leaf

// 	return &cert{c: &c, expiry: leaf.NotAfter}, nil
// }

// func (c *cert) getCert() *tls.Certificate {
// 	now := time.Now().UTC()
// 	dur := now.Sub(c.expiry)

// 	if dur < 14*(24*time.Hour) {
// 		// we consider any certificate that is within 14days as due for renewal(ie, 'expired')
// 		go c.renew()
// 	}

// 	return c.c
// }

// func (c *cert) renew() {
// 	// 1. call letsencrypt and renew cert.
// 	// 2. persist on disk.
// 	// 3. update `c.c` and `c.expiry`
// }

// Run listens on a network address and then calls Serve to handle requests on incoming connections.
// It sets up a server with the parameters provided by o.
//
// The server shuts down cleanly after receiving any terminating signal.
// If the opts supplied include a certificate and key, the server will accept https traffic and also automatically handle http->https redirect.
func Run(eh extendedHandler, o opts) error {
	setRlimit()
	_, _ = maxprocs.Set()

	ctx, cancel := context.WithCancel(context.Background())
	logger := eh.GetLogger().WithCtx(ctx).WithImmediate().WithFields(log.F{"pid": os.Getpid()})

	var tlsConf *tls.Config = nil
	if o.certFile != "" {
		const letsEncryptProductionUrl = "https://acme-v02.api.letsencrypt.org/directory"
		const letsEncryptStagingUrl = "https://acme-staging-v02.api.letsencrypt.org/directory"

		m := &autocert.Manager{
			Client: &acme.Client{DirectoryURL: letsEncryptStagingUrl},
			Cache:  autocert.DirCache("ong-certifiate-dir"),
			Prompt: autocert.AcceptTOS,
			Email:  "example@example.org",
			HostPolicy: autocert.HostWhitelist(
				// todo: replace this with our own function.
				// note: the func(`autocert.HostWhitelist`) does only exact matches. Subdomains, regexp or wildcard will not match.
				//       we should change that.
				"example.org",
				"www.example.org",
			),
		}

		tlsConf = &tls.Config{
			// taken from:
			// https://github.com/golang/crypto/blob/05595931fe9d3f8894ab063e1981d28e9873e2cb/acme/autocert/autocert.go#L228-L234
			NextProtos: []string{
				"h2", "http/1.1", // enable HTTP/2
				acme.ALPNProto, // enable tls-alpn ACME challenges
			},
			GetCertificate: func(info *tls.ClientHelloInfo) (certificate *tls.Certificate, e error) {
				// GetCertificate returns a Certificate based on the given ClientHelloInfo.
				// it is called if `tls.Config.Certificates` is empty.
				//
				// todo: this is where we can renew our certificates if we want.
				// plan;
				//   (a) check if one month has passed.
				//   (b) if it has, call letsencrypt to fetch new certs; maybe in a goroutine.
				//   (c) save that cert to file.
				//   (d) also load it into cache/memory.
				//   (e) if one month is not over, always load certs/key from cache.
				// see:
				//   - golang.org/x/crypto/acme/autocert
				//   - https://github.com/caddyserver/certmagic
				//

				return m.GetCertificate(info)
				// c, err := tls.LoadX509KeyPair(o.certFile, o.keyFile)
				// if err != nil {
				// 	err = ongErrors.Wrap(err)
				// 	logger.Error(err, log.F{"msg": "error loading tls certificate and key."})
				// 	return nil, err
				// }
				// return &c, nil
			},
		}
	}

	server := &http.Server{
		Addr:      o.serverPort,
		TLSConfig: tlsConf,

		// 1. https://blog.simon-frey.eu/go-as-in-golang-standard-net-http-config-will-break-your-production
		// 2. https://blog.cloudflare.com/exposing-go-on-the-internet/
		// 3. https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
		// 4. https://github.com/golang/go/issues/27375
		Handler: http.TimeoutHandler(
			eh,
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
		startPprofServer()
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
	if o.certFile != "" {
		{
			// HTTP(non-tls) LISTERNER:
			redirectSrv := &http.Server{
				Addr:              fmt.Sprintf("127.0.0.1%s", o.httpPort),
				Handler:           middleware.HttpsRedirector(srv.Handler, o.port),
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
					errL = ongErrors.Wrap(errL)
					logger.Error(errL, log.F{"msg": "redirect server, unable to create listener"})
					return
				}

				logger.Info(log.F{
					"msg": fmt.Sprintf("redirect server listening at %s", redirectSrv.Addr),
				})
				errRedirectSrv := redirectSrv.Serve(redirectSrvListener)
				if errRedirectSrv != nil {
					errRedirectSrv = ongErrors.Wrap(errRedirectSrv)
					logger.Error(errRedirectSrv, log.F{"msg": "unable to start redirect server"})
				}
			}()
		}

		{
			// HTTPS(tls) LISTERNER:
			cfg := listenerConfig()
			l, err := cfg.Listen(ctx, o.network, o.serverAddress)
			if err != nil {
				return ongErrors.Wrap(err)
			}
			logger.Info(log.F{
				"msg": fmt.Sprintf("https server listening at %s", o.serverAddress),
			})
			if errS := srv.ServeTLS(l, o.certFile, o.keyFile); errS != nil {
				return ongErrors.Wrap(errS)
			}
		}
	} else {
		cfg := listenerConfig()
		l, err := cfg.Listen(ctx, o.network, o.serverAddress)
		if err != nil {
			return ongErrors.Wrap(err)
		}
		logger.Info(log.F{
			"msg": fmt.Sprintf("http server listening at %s", o.serverAddress),
		})
		if errS := srv.Serve(l); errS != nil {
			return ongErrors.Wrap(errS)
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
