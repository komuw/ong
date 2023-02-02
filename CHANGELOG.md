# Release Notes

Most recent version is listed first.  


## v0.0.30
- Update to Go v1.20: https://github.com/komuw/ong/pull/209

## v0.0.29
- WithCtx should only use the id from context, if that ctx actually contains an Id: https://github.com/komuw/ong/pull/196
- ong/errors; wrap as deep as possible: https://github.com/komuw/ong/pull/199
- ong/errors; add errors.Dwrap: https://github.com/komuw/ong/pull/200
- ong/id, bug fix where ids generated were not always of the requested length; https://github.com/komuw/ong/pull/201
- Do not use math/rand in encryption: https://github.com/komuw/ong/pull/203
- Improve examples: https://github.com/komuw/ong/pull/204
- Do not duplicate session cookies: https://github.com/komuw/ong/pull/206
- Fix changelog versions: https://github.com/komuw/ong/pull/207

## v0.0.28
- ong/id should generate strings of the exact requested length: https://github.com/komuw/ong/pull/192
- Do not quote special characters: https://github.com/komuw/ong/pull/193

## v0.0.27
- Add Get cookie function: https://github.com/komuw/ong/pull/189

## v0.0.26
- Create middleware that adds the "real" client IP address: https://github.com/komuw/ong/pull/187        
  Note that this is on a best effort basis.       
  Finding the true client IP address is a precarious process [1](https://adam-p.ca/blog/2022/03/x-forwarded-for/)      

## v0.0.25
- ong/client: Use roundTripper for logging: https://github.com/komuw/ong/pull/185
- Make most middleware private: https://github.com/komuw/ong/pull/186

## v0.0.24
- Set session cookie only if non-empty: https://github.com/komuw/ong/pull/170
- Add ReloadProtector middleware: https://github.com/komuw/ong/pull/171
- Creating a new route should panic if handler is already wrapped in an ong middleware: https://github.com/komuw/ong/pull/172

## v0.0.23
- ong/client: Add log id http header: https://github.com/komuw/ong/pull/166

## v0.0.22
- Panic/recoverer middleware should include correct stack trace: https://github.com/komuw/ong/pull/164
- Log client address without port: https://github.com/komuw/ong/pull/165

## v0.0.21
- Improve performance of calling Csrf middleware multiple times: https://github.com/komuw/ong/pull/161

## v0.0.20
- Bugfix: When a route conflict is detected, report the correct file & line number: https://github.com/komuw/ong/pull/160

## v0.0.19
- Fix false positive/negative/whatever route conflict: https://github.com/komuw/ong/pull/157

## v0.0.18
- Update documentation

## v0.0.17
- Update documentation

## v0.0.16
- Add support for http sessions: https://github.com/komuw/ong/pull/154
- Add ability to specify a custom 404 handler: https://github.com/komuw/ong/pull/155

## v0.0.15
- Make encrypted cookies more performant: https://github.com/komuw/ong/pull/152

## v0.0.14
- Update documentation: https://github.com/komuw/ong/pull/151

## v0.0.13
- Fix bug in parsing cgroup mem values from files: https://github.com/komuw/ong/pull/148

## v0.0.12
- Prefix errors produced by ong with a constant string: https://github.com/komuw/ong/pull/147
- Try and mitigate cookie replay attacks: https://github.com/komuw/ong/pull/146

## v0.0.11
- Add secure/encrypted cookies: https://github.com/komuw/ong/pull/143

## v0.0.10
- Remove ctx from log.Logger struct: https://github.com/komuw/ong/pull/142

## v0.0.9
- Add password hashing capabilities: https://github.com/komuw/ong/pull/137
- Simplify loadshedding implementation: https://github.com/komuw/ong/pull/138
- Make automax to be a stand-alone package: https://github.com/komuw/ong/pull/139
- Add a router/muxer with a bit more functionality: https://github.com/komuw/ong/pull/140

## v0.0.8
- Improve documentation.

## v0.0.7
- Implement io.ReaderFrom & http.Pusher: https://github.com/komuw/ong/pull/131
- Replace use of net.Ip with net/netip: https://github.com/komuw/ong/pull/132

## v0.0.6
- Improve documentation.

## v0.0.5
- use key derivation in the `enc` ecryption/decryption package: https://github.com/komuw/ong/pull/119
- fix vulnerabilities: https://github.com/komuw/ong/pull/123
- add a http client: https://github.com/komuw/ong/pull/120

## v0.0.4
- add new encryption/decryption package: https://github.com/komuw/ong/pull/118

## v0.0.3
- add an xcontext package: https://github.com/komuw/ong/pull/109
- use latest semgrep-go linter: https://github.com/komuw/ong/pull/111
- add semgrep linter: https://github.com/komuw/ong/pull/113
- add ability to handle csrf tokens in a distributed setting: https://github.com/komuw/ong/pull/112
- redirect csrf failures to same url: https://github.com/komuw/ong/pull/117

## v0.0.2
- automatically set GOMAXPROCS in container environments, using internal package: https://github.com/komuw/ong/pull/106

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
- update github.com/akshayjshah/attest: https://github.com/komuw/ong/pull/93
- update to Go 1.19: https://github.com/komuw/ong/pull/102
- remove rlimit code, go1.19 does automatically: https://github.com/komuw/ong/pull/104
- automatically set GOMEMLIMIT in container environments: https://github.com/komuw/ong/pull/105
