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

const (
	// defaultMaxBodyBytes the value used as the limit for incoming request bodies, if a custom value was not provided.
	//
	// [Nginx] uses a default value of 1MB, [Apache] uses default of 1GB whereas [Haproxy] does not have such a limit.
	//
	// The max size for http [forms] in Go is 10MB. The max size of the entire bible in text form is ~5MB.
	// Thus here, we are going to use the 2 times the default size for forms.
	// Note that; from the [code] and [docs], it looks like; if you set the maxBodyBytes, this also becomes the maxFormSize.
	//
	// [Nginx]: http://nginx.org/en/docs/http/ngx_http_core_module.html#client_max_body_size
	// [Apache]: https://httpd.apache.org/docs/2.4/mod/core.html#limitrequestbody
	// [Haproxy]: https://discourse.haproxy.org/t/how-can-you-configure-the-nginx-client-max-body-size-equivalent-in-haproxy/1690/2
	// [forms]: https://github.com/golang/go/blob/go1.20.3/src/net/http/request.go#L1233-L1235
	// [code]: https://github.com/golang/go/blob/go1.20.3/src/net/http/request.go#L1233-L1235
	// [code]: https://pkg.go.dev/net/http#Request.ParseForm
	defaultMaxBodyBytes = uint64(2 * 10 * 1024 * 1024) // 20MB

	// defaultDrainDuration is used to determine the shutdown duration if a custom one is not provided.
	defaultDrainDuration = 13 * time.Second

	letsEncryptProductionUrl = "https://acme-v02.api.letsencrypt.org/directory"
	letsEncryptStagingUrl    = "https://acme-staging-v02.api.letsencrypt.org/directory"
)

type tlsOpts struct {
	// if certFile is present, tls will be served from certificates on disk.
	certFile string
	keyFile  string
	// if email is present, tls will be served from ACME certifiates.
	email string
	// domain can be a wildcard.
	// However, the certificate issued will NOT be wildcard certs; since letsencrypt only issues wildcard certs via DNS-01 challenge
	// Instead, we'll get a certifiate per subdomain.
	// see; https://letsencrypt.org/docs/faq/#does-let-s-encrypt-issue-wildcard-certificates
	domain string
	// URL of the ACME certificate authority's directory endpoint.
	url string
}

// Opts are the various parameters(optionals) that can be used to configure a HTTP server.
//
// Use either [NewOpts], [DevOpts], [CertOpts], [AcmeOpts] or [LetsEncryptOpts] to get a valid Opts.
type Opts struct {
	port              uint16 // tcp port is a 16bit unsigned integer.
	maxBodyBytes      uint64 // max size of request body allowed.
	readHeaderTimeout time.Duration
	readTimeout       time.Duration
	writeTimeout      time.Duration
	handlerTimeout    time.Duration
	idleTimeout       time.Duration
	drainTimeout      time.Duration
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
// maxBodyBytes is the maximum size in bytes for incoming request bodies. If this is zero, a reasonable default is used.
//
// readHeaderTimeout is the amount of time a server will be allowed to read request headers.
// readTimeout is the maximum duration a server will use for reading the entire request, including the body.
// writeTimeout is the maximum duration before a server times out writes of the response.
// handlerTimeout is the maximum duration that handlers on the server will serve a request before timing out.
// idleTimeout is the maximum amount of time to wait for the next request when keep-alives are enabled.
// drainTimeout is the duration to wait for after receiving a shutdown signal and actually starting to shutdown the server.
// This is important especially in applications running in places like kubernetes.
//
// certFile is a path to a tls certificate.
// keyFile is a path to a tls key.
//
// email is the e-address that will be used if/when procuring certificates from an [ACME] certificate authority, eg [letsencrypt].
// domain is the domain name of your website; it can be an exact domain, subdomain or wildcard.
// acmeURL is the URL of the [ACME] certificate authority's directory endpoint.
//
// If certFile is a non-empty string, this will enable tls using certificates found on disk.
// If email is a non-empty string, this will enable tls using certificates procured from an [ACME] certificate authority.
//
// [ACME]: https://en.wikipedia.org/wiki/Automatic_Certificate_Management_Environment
// [letsencrypt]: https://letsencrypt.org/
func NewOpts(
	port uint16,
	maxBodyBytes uint64,
	readHeaderTimeout time.Duration,
	readTimeout time.Duration,
	writeTimeout time.Duration,
	handlerTimeout time.Duration,
	idleTimeout time.Duration,
	drainTimeout time.Duration,
	certFile string,
	keyFile string,
	email string, // if present, tls will be served from acme certifiates.
	domain string,
	acmeURL string,
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

	if maxBodyBytes <= 0 {
		maxBodyBytes = defaultMaxBodyBytes
	}

	if email != "" && acmeURL == "" {
		acmeURL = letsEncryptProductionUrl
		if os.Getenv("ONG_RUNNING_IN_TESTS") != "" {
			acmeURL = letsEncryptStagingUrl
		}
	}

	return Opts{
		port:              port,
		maxBodyBytes:      maxBodyBytes,
		readHeaderTimeout: readHeaderTimeout,
		readTimeout:       readTimeout,
		writeTimeout:      writeTimeout,
		handlerTimeout:    handlerTimeout,
		idleTimeout:       idleTimeout,
		drainTimeout:      drainTimeout,
		tls: tlsOpts{
			certFile: certFile,
			keyFile:  keyFile,
			email:    email,
			domain:   domain,
			url:      acmeURL,
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

	return withOpts(65081, certFile, keyFile, "", "localhost", "")
}

// CertOpts returns a new Opts that has sensible defaults given certFile & keyFile.
func CertOpts(certFile, keyFile, domain string) Opts {
	return withOpts(443, certFile, keyFile, "", domain, "")
}

// AcmeOpts returns a new Opts that procures certificates from an [ACME] certificate authority.
// Also see [LetsEncryptOpts]
//
// [ACME]: https://en.wikipedia.org/wiki/Automatic_Certificate_Management_Environment
func AcmeOpts(email, domain, acmeURL string) Opts {
	return withOpts(443, "", "", email, domain, acmeURL)
}

// LetsEncryptOpts returns a new Opts that procures certificates from [letsencrypt].
// Also see [AcmeOpts]
//
// [letsencrypt]: https://letsencrypt.org/
func LetsEncryptOpts(email, domain string) Opts {
	return withOpts(443, "", "", email, domain, "")
}

// withOpts returns a new Opts that has sensible defaults given port.
func withOpts(port uint16, certFile, keyFile, email, domain, acmeURL string) Opts {
	// readHeaderTimeout < readTimeout < writeTimeout < handlerTimeout < idleTimeout

	readHeaderTimeout := 1 * time.Second
	readTimeout := readHeaderTimeout + (1 * time.Second)
	writeTimeout := readTimeout + (1 * time.Second)
	handlerTimeout := writeTimeout + (10 * time.Second)
	idleTimeout := handlerTimeout + (100 * time.Second)
	drainTimeout := defaultDrainDuration

	maxBodyBytes := defaultMaxBodyBytes

	return NewOpts(
		port,
		maxBodyBytes,
		readHeaderTimeout,
		readTimeout,
		writeTimeout,
		handlerTimeout,
		idleTimeout,
		drainTimeout,
		certFile,
		keyFile,
		email,
		domain,
		acmeURL,
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

	tlsConf, errTc := getTlsConfig(ctx, h, o, l)
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
			http.MaxBytesHandler(
				h,
				int64(o.maxBodyBytes), // limit in bytes.
			),
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

	sigHandler(server, ctx, cancel, l, o.drainTimeout)

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
				logger.Error("redirect server, unable to create listener", "error", errL)
				return
			}

			slog.NewLogLogger(logger.Handler(), log.LevelImmediate).
				Printf("redirect server listening at %s", redirectSrv.Addr)
			errRedirectSrv := redirectSrv.Serve(redirectSrvListener)
			if errRedirectSrv != nil {
				logger.Error("unable to start redirect server", "error", errRedirectSrv)
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
