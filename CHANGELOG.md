# Release Notes

Most recent version is listed first.  

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
