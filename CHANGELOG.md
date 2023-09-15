# Release Notes

Most recent version is listed first.  


# v0.0.82
- ong/config: Move middleware.ClientIPstrategy to config.ClientIPstrategy: https://github.com/komuw/ong/pull/386

# v0.0.81
- ong/middleware: Add support for CORS allowCredentials: https://github.com/komuw/ong/pull/385

# v0.0.80
- ong/middleware: Validate secretKeys a bit more: https://github.com/komuw/ong/pull/384

# v0.0.79
- ong/middleware: Try and prevent path traversal attacks: https://github.com/komuw/ong/pull/382

# v0.0.78
- ong/middleware: Add 'self' to CSP policy: https://github.com/komuw/ong/pull/379

# v0.0.77
- ong/middleware: Add support for http.NewResponseController: https://github.com/komuw/ong/pull/368
- ong/middleware: Improve formatting of some types: https://github.com/komuw/ong/pull/370
- ong/mux: Remove logger from mux: https://github.com/komuw/ong/pull/371
- ong/mux: Add internal/mux: https://github.com/komuw/ong/pull/372
- ong/config: Create common config options: https://github.com/komuw/ong/pull/374
- ong/config: Add config.secureKey: https://github.com/komuw/ong/pull/376
- ong/server: Serve pprof using the same server as the application: https://github.com/komuw/ong/pull/375

# v0.0.76
- ong/middleware: Configure what percentage of ratelimited or loadshed responses should be logged: https://github.com/komuw/ong/pull/364

# v0.0.75
- ong/middleware: Make more middleware configurable: https://github.com/komuw/ong/pull/362

# v0.0.74
- ong/log: Conform to log/slog: https://github.com/komuw/ong/pull/357
           Eliminate duplicate logIDs
- ong/log: Integrate ong packages with new logger: https://github.com/komuw/ong/pull/358
- ong/log: Improve log perfomance: https://github.com/komuw/ong/pull/359

# v0.0.73
- Remove /ex/exp: https://github.com/komuw/ong/pull/352

# v0.0.72
- ong/log: Use slog from stdlib: https://github.com/komuw/ong/pull/349
- ong/xcontext: Remove the package since Go v1.21 has similar functionality: https://github.com/komuw/ong/pull/350
- Use testing.Testing() where appropriate: https://github.com/komuw/ong/pull/351

# v0.0.71
- Only use test dependencies in test files: https://github.com/komuw/ong/pull/348

# v0.0.70
- ong/middleware: Fix DNS rebinding via http: https://github.com/komuw/ong/compare/issues/337
- ong/server: Dynamically assign port for pprof: https://github.com/komuw/ong/pull/343
- And test util: https://github.com/komuw/ong/pull/344
- ong/middleware: Send Allow http header when we respond with http 405 status code: https://github.com/komuw/ong/pull/345
- ong/middleware: Return http 404 instead of 400 for bad host header: https://github.com/komuw/ong/pull/346
- ong/id: Use crypto/rand in id.New(): https://github.com/komuw/ong/pull/347

# v0.0.69
- ong/server: Bugfix; add ability to use mutual TLS authentication: https://github.com/komuw/ong/pull/335

# v0.0.68
- ong/server: Add ability to use mutual TLS authentication: https://github.com/komuw/ong/pull/334

# v0.0.67
- ong/middleware: Update security headers: https://github.com/komuw/ong/pull/330
- ong: Make sure that the secretKey has some minimum security properties: https://github.com/komuw/ong/pull/329

# v0.0.66
- ong/acme: Bugfix, fetch certficate for subdomain beginning with number: https://github.com/komuw/ong/pull/328

# v0.0.65
- ong/id: Do not use crypto/rand: https://github.com/komuw/ong/pull/322
- ong/middleware: Fix logging of ratelimited/loadshedded requests: https://github.com/komuw/ong/pull/325
- ong/acme: When an acme challenge request is malformed, fail the request: https://github.com/komuw/ong/pull/326

# v0.0.64
- ong/acme: Limit size of certificate allowed for download: https://github.com/komuw/ong/pull/321

# v0.0.63
- ong/middleware: Log unexpected http HOST header: https://github.com/komuw/ong/pull/315
- Update dependencies: https://github.com/komuw/ong/pull/318
- ong/log: Do not duplicate logID: https://github.com/komuw/ong/pull/317
- ong/log: Log source attribute in one line: https://github.com/komuw/ong/pull/319

# v0.0.62
- ong/acme: Check acme.HostPolicy before making request to ACME servers: https://github.com/komuw/ong/pull/313
- ong/acme: add context cancellation: https://github.com/komuw/ong/pull/314

# v0.0.61
- ong/server: Log http.Server.ErrorLog at Info by default: https://github.com/komuw/ong/pull/312

# v0.0.60
- ong/acme: Refuse to fetch certificates for IP addresses: https://github.com/komuw/ong/pull/306
- ong/acme: If fetch certificates fails, log the clientIP and clientFingerPrint: https://github.com/komuw/ong/pull/310
- ong/acme: Add a fastpath(for certs that are already in memory) while fetching certificates: https://github.com/komuw/ong/pull/311

# v0.0.59
- ong/acme: Fix bug where acme certificates were not cached in memory: https://github.com/komuw/ong/pull/304

# v0.0.58
- ong/acme: Tighten check cert validity script: https://github.com/komuw/ong/pull/303

# v0.0.57
- ong/acme: Add own ACME client implementation: https://github.com/komuw/ong/pull/294
- Work around bug in checklocks static analyzer: https://github.com/komuw/ong/pull/298
- Make tests fast by pinging port: https://github.com/komuw/ong/pull/299
- Synchronize automax tests: https://github.com/komuw/ong/pull/300
- Improve rate limit tests: https://github.com/komuw/ong/pull/301
- ong/middleware: Fix superfluous response.WriteHeader call: https://github.com/komuw/ong/pull/302

# v0.0.56
- ong/server: Set appropriate log level for http.Server.ErrorLog: https://github.com/komuw/ong/pull/288
- ong/acme: Move acme handler to ong/middleware: https://github.com/komuw/ong/pull/290
- ong/id: Add uuid support: https://github.com/komuw/ong/pull/292

# v0.0.55
- ong/client: Improve timeouts: https://github.com/komuw/ong/pull/286
- ong/acme: Use one server for ACME and app: https://github.com/komuw/ong/pull/287

# v0.0.54
- ong/middleware: Validate domain in middleware: https://github.com/komuw/ong/pull/283

# v0.0.53
- ong/acme: Add acme server that will handle requests from ACME CA: https://github.com/komuw/ong/pull/281

# v0.0.52
- ong/server: Bugfix; match number of log arguments: https://github.com/komuw/ong/pull/275
- ong/middleware: Add protection against DNS rebinding attacks: https://github.com/komuw/ong/pull/276

# v0.0.51
- ong/acme: Add a http timeout when calling ACME for certificates: https://github.com/komuw/ong/pull/272
- ong/acme: Make certificate management from ACME to be agnostic of the CA: https://github.com/komuw/ong/pull/273

# v0.0.50
- Fix documentation linking: https://github.com/komuw/ong/commit/4cd5d47a3a431d25e84ffb04242d5b57eb2a803e

# v0.0.49
- ong/mux: Add mux Resolve function: https://github.com/komuw/ong/pull/268
- ong/middleware: Use http.Handler as the http middleware instead of http.HandlerFunc: https://github.com/komuw/ong/pull/269
- ong/client: Add optional http timeout: https://github.com/komuw/ong/pull/270
- Use Go cache in CI: https://github.com/komuw/ong/pull/271

## v0.0.48
- Change attest import path: https://github.com/komuw/ong/pull/265

## v0.0.47
- ong/server: Leave http.server.DisableGeneralOptionsHandler at its default value: https://github.com/komuw/ong/pull/255
- ong/middleware: Validate expiry of csrf tokens: https://github.com/komuw/ong/pull/257
- ong/middleware: Add support for PROXY protocol in clientIP: https://github.com/komuw/ong/pull/258
- Add nilness vet check: https://github.com/komuw/ong/pull/259
- ong/server: Add option to restrict size of request bodies: https://github.com/komuw/ong/pull/261
- ong/server: Gracefully handle application termniation in kubernetes: https://github.com/komuw/ong/pull/263
- ong/log: Update to latest exp/slog: https://github.com/komuw/ong/pull/262

## v0.0.46
- ong/cookie: Include TLS fingerprint in encrypted cookies: https://github.com/komuw/ong/pull/250
- ong/log: Update to latest exp/slog: https://github.com/komuw/ong/pull/251

## v0.0.45
- Run all tests in CI: https://github.com/komuw/ong/pull/248

## v0.0.44
- Organise imports: https://github.com/komuw/ong/pull/245
- Create an internal/octx that houses context keys used by multiple ong packages: https://github.com/komuw/ong/pull/246
- ong/middleware: Add support for TLS fingerprinting: https://github.com/komuw/ong/pull/244

## v0.0.43
- ong/middleware: Add precision to ratelimiting: https://github.com/komuw/ong/pull/239

## v0.0.42
- ong/middleware: ClientIP, use remoteAddress if IP is local adress: https://github.com/komuw/ong/pull/238

## v0.0.41
- ong/middleware: Better loadshed calculations: https://github.com/komuw/ong/pull/234
                                              : https://github.com/komuw/ong/pull/237

## v0.0.40
- Detect leaks in tests: https://github.com/komuw/ong/pull/232
- Bugfix; loadshed records latency in milliseconds: https://github.com/komuw/ong/pull/233

## v0.0.39
- Remove pid from logs: https://github.com/komuw/ong/pull/230

## v0.0.38
- Update to latest exp/slog changes: https://github.com/komuw/ong/pull/229

## v0.0.37
- Make gvisor/checklocks analyzer ignore tests: https://github.com/komuw/ong/pull/228

## v0.0.36
- Update to latest exp/slog changes: https://github.com/komuw/ong/pull/226
- Add gvisor/checklocks analyzer: https://github.com/komuw/ong/pull/202

## v0.0.35
- Run integration tests in CI: https://github.com/komuw/ong/pull/225

## v0.0.34
- Create dev certs only if they do not exists or are expired: https://github.com/komuw/ong/pull/224

## v0.0.33
- Remove log.Handler.StdLogger(), upstream slog now has an analogous function: https://github.com/komuw/ong/pull/219

## v0.0.32
- ong/middleware: Loadshedder should not re-order latencies: https://github.com/komuw/ong/pull/218

## v0.0.31
- Bugfix; immediately log when server gets os/interrupt signal: https://github.com/komuw/ong/commit/b9ed83a98e7bba0350a473b668ddc2ba8d4677cd

## v0.0.30
- Update to Go v1.20: https://github.com/komuw/ong/pull/209
- ong/client: Use net.Dialer.ControlContext instead of use net.Dialer.Control: https://github.com/komuw/ong/pull/212
- Re-enable golangci-lint: https://github.com/komuw/ong/pull/214
- ong/log: Use the new stdlib structured logger: https://github.com/komuw/ong/pull/208
- ong/log: Replace custom logger with slog: https://github.com/komuw/ong/pull/215
- ong/middleware; Add a trace middleware: https://github.com/komuw/ong/pull/216

## v0.0.29
- ong/log: WithCtx should only use the id from context, if that ctx actually contains an Id: https://github.com/komuw/ong/pull/196
- ong/errors: wrap as deep as possible: https://github.com/komuw/ong/pull/199
- ong/errors: add errors.Dwrap: https://github.com/komuw/ong/pull/200
- ong/id: bug fix where ids generated were not always of the requested length; https://github.com/komuw/ong/pull/201
- ong/cry: Do not use math/rand in encryption: https://github.com/komuw/ong/pull/203
- Improve examples: https://github.com/komuw/ong/pull/204
- ong/middleware: Do not duplicate session cookies: https://github.com/komuw/ong/pull/206
- Fix changelog versions: https://github.com/komuw/ong/pull/207

## v0.0.28
- ong/id: Should generate strings of the exact requested length: https://github.com/komuw/ong/pull/192
- ong/log: Do not quote special characters: https://github.com/komuw/ong/pull/193

## v0.0.27
- ong/cookie: Add Get cookie function: https://github.com/komuw/ong/pull/189

## v0.0.26
- ong/middleware: Create middleware that adds the "real" client IP address: https://github.com/komuw/ong/pull/187        
  Note that this is on a best effort basis.       
  Finding the true client IP address is a precarious process [1](https://adam-p.ca/blog/2022/03/x-forwarded-for/)      

## v0.0.25
- ong/client: Use roundTripper for logging: https://github.com/komuw/ong/pull/185
- ong/middleware: Make most middleware private: https://github.com/komuw/ong/pull/186

## v0.0.24
- ong/sess: Set session cookie only if non-empty: https://github.com/komuw/ong/pull/170
- ong/middleware: Add ReloadProtector middleware: https://github.com/komuw/ong/pull/171
- ong/mux: Creating a new route should panic if handler is already wrapped in an ong middleware: https://github.com/komuw/ong/pull/172

## v0.0.23
- ong/client: Add log id http header: https://github.com/komuw/ong/pull/166

## v0.0.22
- ong/middleware: Panic/recoverer middleware should include correct stack trace: https://github.com/komuw/ong/pull/164
- ong/middleware: Log client address without port: https://github.com/komuw/ong/pull/165

## v0.0.21
- ong/cry: Improve performance of calling Csrf middleware multiple times: https://github.com/komuw/ong/pull/161

## v0.0.20
- ong/mux: Bugfix: When a route conflict is detected, report the correct file & line number: https://github.com/komuw/ong/pull/160

## v0.0.19
- ong/mux: Fix false positive/negative/whatever route conflict: https://github.com/komuw/ong/pull/157

## v0.0.18
- Update documentation

## v0.0.17
- Update documentation

## v0.0.16
- ong/cookie: Add support for http sessions: https://github.com/komuw/ong/pull/154
- ong/mux: Add ability to specify a custom 404 handler: https://github.com/komuw/ong/pull/155

## v0.0.15
- ong/cookie: Make encrypted cookies more performant: https://github.com/komuw/ong/pull/152

## v0.0.14
- Update documentation: https://github.com/komuw/ong/pull/151

## v0.0.13
- ong/automax: Fix bug in parsing cgroup mem values from files: https://github.com/komuw/ong/pull/148

## v0.0.12
- ong/errors: Prefix errors produced by ong with a constant string: https://github.com/komuw/ong/pull/147
- ong/cookie: Try and mitigate cookie replay attacks: https://github.com/komuw/ong/pull/146

## v0.0.11
- ong/cookie; Add secure/encrypted cookies: https://github.com/komuw/ong/pull/143

## v0.0.10
- ong/log: Remove ctx from log.Logger struct: https://github.com/komuw/ong/pull/142

## v0.0.9
- ong/cry: Add password hashing capabilities: https://github.com/komuw/ong/pull/137
- ong/middleware: Simplify loadshedding implementation: https://github.com/komuw/ong/pull/138
- ong/automax: Make automax to be a stand-alone package: https://github.com/komuw/ong/pull/139
- ong/mux: Add a router/muxer with a bit more functionality: https://github.com/komuw/ong/pull/140

## v0.0.8
- Improve documentation.

## v0.0.7
- ong/middleware: Implement io.ReaderFrom & http.Pusher: https://github.com/komuw/ong/pull/131
- ong/client: Replace use of net.Ip with net/netip: https://github.com/komuw/ong/pull/132

## v0.0.6
- Improve documentation.

## v0.0.5
- ong/cry: use key derivation in the `enc` ecryption/decryption package: https://github.com/komuw/ong/pull/119
- fix vulnerabilities: https://github.com/komuw/ong/pull/123
- ong/client: add a http client: https://github.com/komuw/ong/pull/120

## v0.0.4
- ong/cry: add new encryption/decryption package: https://github.com/komuw/ong/pull/118

## v0.0.3
- ong/xcontext: add an xcontext package: https://github.com/komuw/ong/pull/109
- use latest semgrep-go linter: https://github.com/komuw/ong/pull/111
- add semgrep linter: https://github.com/komuw/ong/pull/113
- ong/middleware: add ability to handle csrf tokens in a distributed setting: https://github.com/komuw/ong/pull/112
- ong/middleware: redirect csrf failures to same url: https://github.com/komuw/ong/pull/117

## v0.0.2
- ong/automax: automatically set GOMAXPROCS in container environments, using internal package: https://github.com/komuw/ong/pull/106

## v0.0.1
- added some middlewares: https://github.com/komuw/ong/pull/22
- add build/test cache: https://github.com/komuw/ong/pull/24
- harmonize timeouts: https://github.com/komuw/ong/pull/25
- add panic middleware: https://github.com/komuw/ong/pull/26
- cookies: https://github.com/komuw/ong/pull/27
- csrf middleware: https://github.com/komuw/ong/pull/32
- cors middleware: https://github.com/komuw/ong/pull/33
- gzip middleware: https://github.com/komuw/ong/pull/36
- errors: https://github.com/komuw/ong/commit/2603c06ca1257d75fb170872124b2afd81eb3f3e
- logger: https://github.com/komuw/ong/pull/39
- logging middleware: https://github.com/komuw/ong/pull/41
- quality of life improvements: https://github.com/komuw/ong/pull/45
- add unique id generator: https://github.com/komuw/ong/pull/50
- try mitigate breach attack: https://github.com/komuw/ong/pull/51
- add load shedding: https://github.com/komuw/ong/pull/52
- fix memory leak in tests: https://github.com/komuw/ong/pull/53
- add ratelimiter: https://github.com/komuw/ong/pull/55
- add naive mux: https://github.com/komuw/ong/pull/57
- handle tls: https://github.com/komuw/ong/pull/58
- expvar metrics: https://github.com/komuw/ong/pull/64
- fix some races: https://github.com/komuw/ong/pull/66
- resuse address/port for pprof and redirect servers: https://github.com/komuw/ong/pull/67
- rename: https://github.com/komuw/ong/pull/68
- make some updates to circular buffer: https://github.com/komuw/ong/pull/71
- use acme for certificates: https://github.com/komuw/ong/pull/69
- issues/73: bind on 0.0.0.0 or localhost conditionally: https://github.com/komuw/ong/pull/74
- redirect IP to domain: https://github.com/komuw/ong/pull/75
- dont require csrf for POST requests that have no cookies and arent http auth: https://github.com/komuw/ong/pull/77
- remove http: https://github.com/komuw/ong/pull/79
- make the redirector a proper middleware: https://github.com/komuw/ong/pull/80
- bugfix, gzip error: https://github.com/komuw/ong/pull/82
- gzip almost everthing: https://github.com/komuw/ong/pull/83
- pass logger as an arg to the middlewares: https://github.com/komuw/ong/pull/84
- disable gzip: https://github.com/komuw/ong/pull/86
- a more efficient error stack trace: https://github.com/komuw/ong/pull/87
- update go.akshayshah.org/attest: https://github.com/komuw/ong/pull/93
- update to Go 1.19: https://github.com/komuw/ong/pull/102
- remove rlimit code, go1.19 does automatically: https://github.com/komuw/ong/pull/104
- automatically set GOMEMLIMIT in container environments: https://github.com/komuw/ong/pull/105
