// Package config provides various parameters(configuration optionals) that can be used to configure ong.
package config

import (
	"fmt"
	"log/slog"
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

// ClientIPstrategy is a middleware option that describes the strategy to use when fetching the client's IP address.
type ClientIPstrategy = clientip.ClientIPstrategy

// Opts are the various parameters(optionals) that can be used to configure ong.
//
// Use either [New] or [WithOpts] to get a valid Opts. TODO:
type Opts struct {
	// middlewareOpts are parameters that are used by middleware.
	middlewareOpts
}

// TODO: string & go string for Opts.

// TODO: docs
func New(
	// middleware
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
	// server
) Opts {
	return Opts{
		NewMiddlewareOpts(
			domain,
			httpsPort,
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
	return Opts{
		WithMiddlewareOpts(
			domain,
			httpsPort,
			secretKey,
			strategy,
			logger,
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
  domain: %s
  httpsPort: %d
  secretKey: %s
  strategy: %v
  l: %v
  rateShedSamplePercent: %v
  rateLimit: %v
  loadShedSamplingPeriod: %v
  loadShedMinSampleSize: %v
  loadShedBreachLatency: %v
  allowedOrigins: %v
  allowedMethods: %v
  allowedHeaders: %v
  corsCacheDuration: %v
  csrfTokenDuration: %v
  sessionCookieDuration: %v
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
