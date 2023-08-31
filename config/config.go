// Package config provides various parameters(configuration optionals) that can be used to configure ong.
package config

import (
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/komuw/ong/internal/acme"
	"github.com/komuw/ong/internal/clientip"
	"github.com/komuw/ong/internal/key"
)

// logging
const (
	// DefaultRateShedSamplePercent is the percentage of rate limited or loadshed responses that will be logged as errors, by default.
	DefaultRateShedSamplePercent = 10
)

// ratelimit
const (
	// DefaultRateLimit is the maximum requests allowed (from one IP address) per second, by default.
	//
	// The figure chosen here is because;
	// [github] uses a rate limit of 1 reqs/sec (5_000 reqs/hr).
	// [twitter] uses 1 reqs/sec (900 reqs/15mins).
	// [stripe] uses 100 reqs/sec.
	//
	// [github]: https://docs.github.com/en/developers/apps/building-github-apps/rate-limits-for-github-apps
	// [twitter]: https://developer.twitter.com/en/docs/twitter-api/rate-limits
	// [stripe]: https://stripe.com/docs/rate-limits
	DefaultRateLimit = 100.00
)

// loadshed
const (
	// DefaultLoadShedSamplingPeriod is the duration over which we calculate response latencies by default.
	DefaultLoadShedSamplingPeriod = 12 * time.Minute
	// DefaultLoadShedMinSampleSize is the minimum number of past requests that have to be available, in the last `loadShedSamplingPeriod` for us to make a decision, by default.
	// If there were fewer requests(than `loadShedMinSampleSize`) in the `loadShedSamplingPeriod`, then we do decide to let things continue without load shedding.
	DefaultLoadShedMinSampleSize = 50
	// DefaultLoadShedBreachLatency is the p99 latency at which point we start dropping requests, by default.
	//
	// The value chosen here is because;
	// The wikipedia [monitoring] dashboards are public.
	// In there we can see that the p95 [response] times for http GET requests is ~700ms, & the p95 response times for http POST requests is ~900ms.
	// Thus, we'll use a `loadShedBreachLatency` of ~700ms. We hope we can do better than wikipedia(chuckle emoji.)
	//
	// [monitoring]: https://grafana.wikimedia.org/?orgId=1
	// [response]: https://grafana.wikimedia.org/d/RIA1lzDZk/application-servers-red?orgId=1
	DefaultLoadShedBreachLatency = 700 * time.Millisecond
)

// cors
const (
	// DefaultCorsCacheDuration is the length in time that preflight responses will be cached by default.
	// 2hrs is chosen since that is the maximum for chromium based browsers.
	// Firefox had a maximum of 24hrs as at the time of writing.
	DefaultCorsCacheDuration = 2 * time.Hour
)

// csrf
const (
	// DefaultCsrfCookieDuration is the duration that csrf cookie will be valid for by default.
	//
	// At the time of writing; gorilla/csrf uses 12hrs, django uses 1yr & gofiber/fiber uses 1hr.
	DefaultCsrfCookieDuration = 12 * time.Hour
)

// session
const (
	// DefaultSessionCookieDuration is the duration that session cookie will be valid for by default.
	// [django] uses a value of 2 weeks by default.
	//
	// [django]: https://docs.djangoproject.com/en/4.1/ref/settings/#session-cookie-age
	DefaultSessionCookieDuration = 14 * time.Hour
)

// TODO: export this??
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
	defaultMaxBodyBytes   = uint64(2 * 10 * 1024 * 1024) // 20MB
	defaultServerLogLevel = slog.LevelInfo

	// defaultDrainDuration is used to determine the shutdown duration if a custom one is not provided.
	defaultDrainDuration = 13 * time.Second

	letsEncryptProductionUrl = "https://acme-v02.api.letsencrypt.org/directory"
	letsEncryptStagingUrl    = "https://acme-staging-v02.api.letsencrypt.org/directory"
)

// ClientIPstrategy is a middleware option that describes the strategy to use when fetching the client's IP address.
type ClientIPstrategy = clientip.ClientIPstrategy

// Opts are the various parameters(optionals) that can be used to configure ong.
//
// Use either [New] or [WithOpts] to get a valid Opts. TODO:
type Opts struct {
	// middlewareOpts are parameters that are used by middleware.
	middlewareOpts
	// serverOpts are parameters that are used by server.
	serverOpts
}

// String implements [fmt.Stringer]
func (o Opts) String() string {
	return fmt.Sprintf(`Opts{
  middlewareOpts: %v
  serverOpts: %v
}`,
		o.middlewareOpts,
		o.serverOpts,
	)
}

// GoString implements [fmt.GoStringer]
func (o Opts) GoString() string {
	return o.String()
}

// TODO: docs
func New(
	// common
	domain string,
	port uint16,

	// middleware
	secretKey string,
	strategy ClientIPstrategy,
	logger *slog.Logger,
	rateShedSamplePercent int,
	rateLimit float64,
	loadShedSamplingPeriod time.Duration,
	loadShedMinSampleSize int,
	loadShedBreachLatency time.Duration,
	allowedOrigins []string,
	allowedMethods []string,
	allowedHeaders []string,
	corsCacheDuration time.Duration,
	csrfTokenDuration time.Duration,
	sessionCookieDuration time.Duration,
	// server
	maxBodyBytes uint64,
	serverLogLevel slog.Level,
	readHeaderTimeout time.Duration,
	readTimeout time.Duration,
	writeTimeout time.Duration,
	idleTimeout time.Duration,
	drainTimeout time.Duration,
	certFile string,
	keyFile string,
	acmeEmail string, // if present, tls will be served from acme certificates.
	acmeDirectoryUrl string,
	clientCertificatePool *x509.CertPool,
) Opts {
	return Opts{
		middlewareOpts: NewMiddlewareOpts(
			domain,
			port,
			secretKey,
			strategy,
			logger,
			rateShedSamplePercent,
			rateLimit,
			loadShedSamplingPeriod,
			loadShedMinSampleSize,
			loadShedBreachLatency,
			allowedOrigins,
			allowedMethods,
			allowedHeaders,
			corsCacheDuration,
			csrfTokenDuration,
			sessionCookieDuration,
		),
		serverOpts: NewServerOpts(
			port,
			maxBodyBytes,
			serverLogLevel,
			readHeaderTimeout,
			readTimeout,
			writeTimeout,
			idleTimeout,
			drainTimeout,
			certFile,
			keyFile,
			acmeEmail,
			domain,
			acmeDirectoryUrl,
			clientCertificatePool,
		),
	}
}

// TODO: docs
func WithOpts(
	// middleware
	domain string,
	httpsPort uint16,
	secretKey string,
	strategy ClientIPstrategy,
	logger *slog.Logger,
	// server
) Opts {
	certFile, keyFile := createDevCertKey(logger)

	return Opts{
		middlewareOpts: WithMiddlewareOpts(
			domain,
			httpsPort,
			secretKey,
			strategy,
			logger,
		),
		serverOpts: withServerOpts(
			httpsPort,
			certFile,
			keyFile,
			"",
			domain,
			"",
		),
	}
}

// DevOpts returns a new Opts that has sensible defaults, especially for dev environments.
// It also automatically creates & configures the developer TLS certificates/key.
func DevOpts(logger *slog.Logger, secretKey string) Opts {
	domain := "localhost"
	httpsPort := uint16(65081)
	certFile, keyFile := createDevCertKey(logger)

	return Opts{
		middlewareOpts: WithMiddlewareOpts(
			domain,
			httpsPort,
			secretKey,
			clientip.DirectIpStrategy,
			logger,
		),
		serverOpts: withServerOpts(
			httpsPort,
			certFile,
			keyFile,
			"",
			domain,
			"",
		),
	}
}

// CertOpts returns a new Opts that has sensible defaults given certFile & keyFile.
func CertOpts(
	// middleware
	domain string,
	secretKey string,
	strategy ClientIPstrategy,
	logger *slog.Logger,
	// server
	certFile string,
	keyFile string,
) Opts {
	httpsPort := uint16(443)
	return Opts{
		middlewareOpts: WithMiddlewareOpts(
			domain,
			httpsPort,
			secretKey,
			strategy,
			logger,
		),
		serverOpts: withServerOpts(
			httpsPort,
			certFile,
			keyFile,
			"",
			domain,
			"",
		),
	}
}

// AcmeOpts returns a new Opts that procures certificates from an [ACME] certificate authority.
// Also see [LetsEncryptOpts]
//
// [ACME]: https://en.wikipedia.org/wiki/Automatic_Certificate_Management_Environment
func AcmeOpts(
	// middleware
	domain string,
	secretKey string,
	strategy ClientIPstrategy,
	logger *slog.Logger,
	// server
	acmeEmail string,
	acmeDirectoryUrl string,
) Opts {
	httpsPort := uint16(443)
	return Opts{
		middlewareOpts: WithMiddlewareOpts(
			domain,
			httpsPort,
			secretKey,
			strategy,
			logger,
		),
		serverOpts: withServerOpts(
			httpsPort,
			"",
			"",
			acmeEmail,
			domain,
			acmeDirectoryUrl,
		),
	}
}

// LetsEncryptOpts returns a new Opts that procures certificates from [letsencrypt].
// Also see [AcmeOpts]
//
// [letsencrypt]: https://letsencrypt.org/
func LetsEncryptOpts(
	// middleware
	domain string,
	secretKey string,
	strategy ClientIPstrategy,
	logger *slog.Logger,
	// server
	acmeEmail string,
) Opts {
	httpsPort := uint16(443)
	return Opts{
		middlewareOpts: WithMiddlewareOpts(
			domain,
			httpsPort,
			secretKey,
			strategy,
			logger,
		),
		serverOpts: withServerOpts(
			httpsPort,
			"",
			"",
			acmeEmail,
			domain,
			"",
		),
	}
}

type middlewareOpts struct {
	Domain    string
	HttpsPort uint16
	SecretKey string
	Strategy  ClientIPstrategy
	Logger    *slog.Logger

	// logger
	RateShedSamplePercent int

	// ratelimit
	RateLimit float64

	// loadshed
	LoadShedSamplingPeriod time.Duration
	LoadShedMinSampleSize  int
	LoadShedBreachLatency  time.Duration

	// cors
	AllowedOrigins    []string
	AllowedMethods    []string
	AllowedHeaders    []string
	CorsCacheDuration time.Duration

	// csrf
	CsrfTokenDuration time.Duration

	// session
	SessionCookieDuration time.Duration
}

// String implements [fmt.Stringer]
func (m middlewareOpts) String() string {
	return fmt.Sprintf(`middlewareOpts{
  Domain: %s
  HttpsPort: %d
  SecretKey: %s
  Strategy: %v
  Logger: %v
  RateShedSamplePercent: %v
  RateLimit: %v
  LoadShedSamplingPeriod: %v
  LoadShedMinSampleSize: %v
  LoadShedBreachLatency: %v
  AllowedOrigins: %v
  AllowedMethods: %v
  AllowedHeaders: %v
  CorsCacheDuration: %v
  CsrfTokenDuration: %v
  SessionCookieDuration: %v
}`,
		m.Domain,
		m.HttpsPort,
		fmt.Sprintf("%s<REDACTED>", string(m.SecretKey[0])),
		m.Strategy,
		m.Logger,
		m.RateShedSamplePercent,
		m.RateLimit,
		m.LoadShedSamplingPeriod,
		m.LoadShedMinSampleSize,
		m.LoadShedBreachLatency,
		m.AllowedOrigins,
		m.AllowedMethods,
		m.AllowedHeaders,
		m.CorsCacheDuration,
		m.CsrfTokenDuration,
		m.SessionCookieDuration,
	)
}

// GoString implements [fmt.GoStringer]
func (m middlewareOpts) GoString() string {
	return m.String()
}

// TODO: un-export this.
//
// NewMiddlewareOpts returns a new Opts.
// It panics on error.
//
// domain is the domain name of your website. It can be an exact domain, subdomain or wildcard.
//
// httpsPort is the tls port where http requests will be redirected to.
//
// secretKey is used for securing signed data.
// It should be unique & kept secret.
// If it becomes compromised, generate a new one and restart your application using the new one.
//
// strategy is the algorithm to use when fetching the client's IP address; see [ClientIPstrategy].
// It is important to choose your strategy carefully, see the warning in [ClientIP].
//
// logger is an [slog.Logger] that will be used for logging.
//
// rateShedSamplePercent is the percentage of rate limited or loadshed responses that will be logged as errors. If it is less than 0, [config.DefaultRateShedSamplePercent] is used instead.
//
// rateLimit is the maximum requests allowed (from one IP address) per second. If it is les than 1.0, [config.DefaultRateLimit] is used instead.
//
// loadShedSamplingPeriod is the duration over which we calculate response latencies for purposes of determining whether to loadshed. If it is less than 1second, [config.DefaultLoadShedSamplingPeriod] is used instead.
// loadShedMinSampleSize is the minimum number of past requests that have to be available, in the last [loadShedSamplingPeriod] for us to make a decision, by default.
// If there were fewer requests(than [loadShedMinSampleSize]) in the [loadShedSamplingPeriod], then we do decide to let things continue without load shedding.
// If it is less than 1, [config.DefaultLoadShedMinSampleSize] is used instead.
// loadShedBreachLatency is the p99 latency at which point we start dropping(loadshedding) requests. If it is less than 1nanosecond, [config.DefaultLoadShedBreachLatency] is used instead.
//
// allowedOrigins, allowedMethods, allowedHeaders & corsCacheDuration are used by the CORS middleware.
// If allowedOrigins is nil, all origins are allowed. You can also use []string{"*"} to allow all.
// If allowedMethods is nil, "GET", "POST", "HEAD" are allowed. Use []string{"*"} to allow all.
// If allowedHeaders is nil, "Origin", "Accept", "Content-Type", "X-Requested-With" are allowed. Use []string{"*"} to allow all.
// corsCacheDuration is the duration that preflight responses will be cached. If it is less than 1second, [config.DefaultCorsCacheDuration] is used instead.
//
// csrfTokenDuration is the duration that csrf cookie will be valid for. If it is less than 1second, [config.DefaultCsrfCookieDuration] is used instead.
//
// sessionCookieDuration is the duration that session cookie will be valid. If it is less than 1second, [config.DefaultSessionCookieDuration] is used instead.
//
// Also see [WithOpts].
//
// [ACME]: https://en.wikipedia.org/wiki/Automatic_Certificate_Management_Environment
// [letsencrypt]: https://letsencrypt.org/
func NewMiddlewareOpts(
	domain string,
	httpsPort uint16,
	secretKey string,
	strategy ClientIPstrategy,
	logger *slog.Logger,
	rateShedSamplePercent int,
	rateLimit float64,
	loadShedSamplingPeriod time.Duration,
	loadShedMinSampleSize int,
	loadShedBreachLatency time.Duration,
	allowedOrigins []string,
	allowedMethods []string,
	allowedHeaders []string,
	corsCacheDuration time.Duration,
	csrfTokenDuration time.Duration,
	sessionCookieDuration time.Duration,
) middlewareOpts {
	if err := acme.Validate(domain); err != nil {
		panic(err)
	}

	if strings.Contains(domain, "*") {
		// remove the `*` and `.`
		domain = domain[2:]
	}

	if err := key.IsSecure(secretKey); err != nil {
		panic(err)
	}

	return middlewareOpts{
		Domain:    domain,
		HttpsPort: httpsPort,
		SecretKey: secretKey,
		Strategy:  strategy,
		Logger:    logger,

		// logger
		RateShedSamplePercent: rateShedSamplePercent,

		// ratelimiter
		RateLimit: rateLimit,

		// loadshed
		LoadShedSamplingPeriod: loadShedSamplingPeriod,
		LoadShedMinSampleSize:  loadShedMinSampleSize,
		LoadShedBreachLatency:  loadShedBreachLatency,

		// cors
		AllowedOrigins:    allowedOrigins,
		AllowedMethods:    allowedMethods,
		AllowedHeaders:    allowedHeaders,
		CorsCacheDuration: corsCacheDuration,

		// csrf
		CsrfTokenDuration: csrfTokenDuration,

		// session
		SessionCookieDuration: sessionCookieDuration,
	}
}

// TODO: un-export this.
//
// WithMiddlewareOpts returns a new Opts that has sensible defaults.
// It panics on error.
//
// See [New] for extra documentation.
func WithMiddlewareOpts(
	domain string,
	httpsPort uint16,
	secretKey string,
	strategy ClientIPstrategy,
	logger *slog.Logger,
) middlewareOpts {
	return NewMiddlewareOpts(
		domain,
		httpsPort,
		secretKey,
		strategy,
		logger,
		DefaultRateShedSamplePercent,
		DefaultRateLimit,
		DefaultLoadShedSamplingPeriod,
		DefaultLoadShedMinSampleSize,
		DefaultLoadShedBreachLatency,
		nil,
		nil,
		nil,
		DefaultCorsCacheDuration,
		DefaultCsrfCookieDuration,
		DefaultSessionCookieDuration,
	)
}

type tlsOpts struct {
	// if certFile is present, tls will be served from certificates on disk.
	CertFile string
	KeyFile  string
	// if acmeEmail is present, tls will be served from ACME certificates.
	AcmeEmail string
	// Domain can be a wildcard.
	// However, the certificate issued will NOT be wildcard certs; since letsencrypt only issues wildcard certs via DNS-01 challenge
	// Instead, we'll get a certificate per subdomain.
	// see; https://letsencrypt.org/docs/faq/#does-let-s-encrypt-issue-wildcard-certificates
	Domain string
	// URL of the ACME certificate authority's directory endpoint.
	AcmeDirectoryUrl      string
	ClientCertificatePool *x509.CertPool
}

// String implements [fmt.Stringer]
func (t tlsOpts) String() string {
	return fmt.Sprintf(`tlsOpts{
  CertFile: %v
  KeyFile: %v
  AcmeEmail: %v
  Domain: %v
  AcmeDirectoryUrl: %v
  ClientCertificatePool: %v
}`,
		t.CertFile,
		t.KeyFile,
		t.AcmeEmail,
		t.Domain,
		t.AcmeDirectoryUrl,
		t.ClientCertificatePool,
	)
}

// GoString implements [fmt.GoStringer]
func (t tlsOpts) GoString() string {
	return t.String()
}

// serverOpts are the various parameters(optionals) that can be used to configure a HTTP server.
//
// Use either [NewOpts], [DevOpts], [CertOpts], [AcmeOpts] or [LetsEncryptOpts] to get a valid Opts. // TODO:
type serverOpts struct {
	port              uint16 // tcp port is a 16bit unsigned integer.
	MaxBodyBytes      uint64 // max size of request body allowed.
	ServerLogLevel    slog.Level
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	DrainTimeout      time.Duration

	Tls tlsOpts

	// the following ones are created automatically
	Host          string
	ServerPort    string
	ServerAddress string
	Network       string
	HttpPort      string
	pprofPort     string // TODO: remove this
}

// TODO: un-export this.
//
// NewServerOpts returns a new Opts.
//
// port is the TLS port at which the server should listen on.
//
// maxBodyBytes is the maximum size in bytes for incoming request bodies. If this is zero, a reasonable default is used.
//
// serverLogLevel is the log level of the logger that will be passed into [http.Server.ErrorLog]
//
// readHeaderTimeout is the amount of time a server will be allowed to read request headers.
// readTimeout is the maximum duration a server will use for reading the entire request, including the body.
// writeTimeout is the maximum duration before a server times out writes of the response.
// idleTimeout is the maximum amount of time to wait for the next request when keep-alives are enabled.
// drainTimeout is the duration to wait for after receiving a shutdown signal and actually starting to shutdown the server.
// This is important especially in applications running in places like kubernetes.
//
// certFile is a path to a tls certificate.
// keyFile is a path to a tls key.
//
// acmeEmail is the e-address that will be used if/when procuring certificates from an [ACME] certificate authority, eg [letsencrypt].
// domain is the domain name of your website; it can be an exact domain, subdomain or wildcard.
// acmeDirectoryUrl is the URL of the [ACME] certificate authority's directory endpoint.
//
// clientCertificatePool is an [x509.CertPool], that will be used to verify client certificates.
// Use this option if you would like to perform mutual TLS authentication.
// The given pool will be used as is, without modification.
//
// If certFile is a non-empty string, this will enable tls using certificates found on disk.
// If acmeEmail is a non-empty string, this will enable tls using certificates procured from an [ACME] certificate authority.
//
// [ACME]: https://en.wikipedia.org/wiki/Automatic_Certificate_Management_Environment
// [letsencrypt]: https://letsencrypt.org/
func NewServerOpts(
	port uint16,
	maxBodyBytes uint64,
	serverLogLevel slog.Level,
	readHeaderTimeout time.Duration,
	readTimeout time.Duration,
	writeTimeout time.Duration,
	idleTimeout time.Duration,
	drainTimeout time.Duration,
	certFile string,
	keyFile string,
	acmeEmail string, // if present, tls will be served from acme certificates.
	domain string,
	acmeDirectoryUrl string,
	clientCertificatePool *x509.CertPool,
) serverOpts {
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
	pprofPort := httpPort - 1

	if maxBodyBytes <= 0 {
		maxBodyBytes = defaultMaxBodyBytes
	}

	if acmeEmail != "" && acmeDirectoryUrl == "" {
		acmeDirectoryUrl = letsEncryptProductionUrl
		if os.Getenv("ONG_RUNNING_IN_TESTS") != "" {
			acmeDirectoryUrl = letsEncryptStagingUrl
		}
	}

	return serverOpts{
		port:              port,
		MaxBodyBytes:      maxBodyBytes,
		ServerLogLevel:    serverLogLevel,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		DrainTimeout:      drainTimeout,

		Tls: tlsOpts{
			CertFile:              certFile,
			KeyFile:               keyFile,
			AcmeEmail:             acmeEmail,
			Domain:                domain,
			AcmeDirectoryUrl:      acmeDirectoryUrl,
			ClientCertificatePool: clientCertificatePool,
		},

		// this ones are created automatically
		Host:          host,
		ServerPort:    serverPort,
		ServerAddress: serverAddress,
		Network:       "tcp",
		HttpPort:      fmt.Sprintf(":%d", httpPort),
		pprofPort:     fmt.Sprintf("%d", pprofPort),
	}
}

func withServerOpts(port uint16, certFile, keyFile, acmeEmail, domain, acmeDirectoryUrl string) serverOpts {
	readHeaderTimeout := 1 * time.Second
	readTimeout := readHeaderTimeout + (1 * time.Second)
	writeTimeout := readTimeout + (1 * time.Second)
	handlerTimeout := writeTimeout + (10 * time.Second)
	idleTimeout := handlerTimeout + (100 * time.Second)
	drainTimeout := defaultDrainDuration

	maxBodyBytes := defaultMaxBodyBytes
	serverLogLevel := defaultServerLogLevel
	return NewServerOpts(
		port,
		maxBodyBytes,
		serverLogLevel,
		readHeaderTimeout,
		readTimeout,
		writeTimeout,
		idleTimeout,
		drainTimeout,
		certFile,
		keyFile,
		acmeEmail,
		domain,
		acmeDirectoryUrl,
		nil,
	)
}

// String implements [fmt.Stringer]
func (s serverOpts) String() string {
	return fmt.Sprintf(`serverOpts{
  port: %v
  MaxBodyBytes: %v
  ServerLogLevel: %v
  ReadHeaderTimeout: %v
  ReadTimeout: %v
  WriteTimeout: %v
  IdleTimeout: %v
  DrainTimeout: %v
  Tls: %v
  Host: %v
  ServerPort: %v
  ServerAddress: %v
  Network: %v
  HttpPort: %v
}`,
		s.port,
		s.MaxBodyBytes,
		s.ServerLogLevel,
		s.ReadHeaderTimeout,
		s.ReadTimeout,
		s.WriteTimeout,
		s.IdleTimeout,
		s.DrainTimeout,
		s.Tls,
		s.Host,
		s.ServerPort,
		s.ServerAddress,
		s.Network,
		s.HttpPort,
	)
}

// GoString implements [fmt.GoStringer]
func (s serverOpts) GoString() string {
	return s.String()
}

// Equal compares two Opts for equality.
// It was added for testing purposes.
func (o Opts) Equal(other Opts) bool {
	{
		if o.serverOpts != other.serverOpts {
			return false
		}
	}

	{
		if o.Domain != other.Domain {
			return false
		}
		if o.HttpsPort != other.HttpsPort {
			return false
		}
		if o.SecretKey != other.SecretKey {
			return false
		}
		if o.Strategy != other.Strategy {
			return false
		}
		if o.Logger != other.Logger {
			return false
		}

		if o.RateShedSamplePercent != other.RateShedSamplePercent {
			return false
		}
		if o.RateLimit != other.RateLimit {
			return false
		}

		if o.LoadShedSamplingPeriod != other.LoadShedSamplingPeriod {
			return false
		}
		if o.LoadShedMinSampleSize != other.LoadShedMinSampleSize {
			return false
		}
		if o.LoadShedBreachLatency != other.LoadShedBreachLatency {
			return false
		}

		{
			if !slices.Equal(o.middlewareOpts.AllowedOrigins, other.middlewareOpts.AllowedOrigins) {
				return false
			}
			if !slices.Equal(o.middlewareOpts.AllowedMethods, other.middlewareOpts.AllowedMethods) {
				return false
			}
			if !slices.Equal(o.middlewareOpts.AllowedHeaders, other.middlewareOpts.AllowedHeaders) {
				return false
			}
			if o.CorsCacheDuration != other.CorsCacheDuration {
				return false
			}
		}

		if o.CsrfTokenDuration != other.CsrfTokenDuration {
			return false
		}
		if o.SessionCookieDuration != other.SessionCookieDuration {
			return false
		}
	}
	return true
}
