package middleware

import (
	"net/http"

	"github.com/komuw/ong/internal/clientip"
)

// Some of the code here is inspired by(or taken from):
//
//	(a) https://github.com/realclientip/realclientip-go whose license(BSD Zero Clause License) can be found here: https://github.com/realclientip/realclientip-go/blob/v1.0.0/LICENSE
//

// ClientIPstrategy is a middleware option that describes the strategy to use when fetching the client's IP address.
type ClientIPstrategy = clientip.ClientIPstrategy

const (
	// DirectIpStrategy derives the client IP from [http.Request.RemoteAddr].
	// It should be used if the server accepts direct connections, rather than through a proxy.
	//
	// See the warning in [ClientIP]
	DirectIpStrategy = clientip.DirectIpStrategy

	// LeftIpStrategy derives the client IP from the leftmost valid & non-private IP address in the `X-Fowarded-For` or `Forwarded` header.
	//
	// See the warning in [ClientIP]
	LeftIpStrategy = clientip.LeftIpStrategy

	// RightIpStrategy derives the client IP from the rightmost valid & non-private IP address in the `X-Fowarded-For` or `Forwarded` header.
	//
	// See the warning in [ClientIP]
	RightIpStrategy = clientip.RightIpStrategy

	// ProxyStrategy derives the client IP from the [PROXY protocol v1].
	// This should be used when your application is behind a TCP proxy that uses the v1 PROXY protocol.
	//
	// 	See the warning in [ClientIP]
	//
	// [PROXY protocol v1]: https://www.haproxy.org/download/2.8/doc/proxy-protocol.txt
	ProxyStrategy = clientip.ProxyStrategy
)

// SingleIpStrategy derives the client IP from http header headerName.
//
// headerName MUST NOT be either `X-Forwarded-For` or `Forwarded`.
// It can be something like `CF-Connecting-IP`, `Fastly-Client-IP`, `Fly-Client-IP`, etc; depending on your usecase.
//
// See the warning in [ClientIP]
func SingleIpStrategy(headerName string) ClientIPstrategy {
	return ClientIPstrategy(headerName)
}

// ClientIP returns the "real" client IP address. This will be based on the [ClientIPstrategy] that you chose.
//
// Warning: This should be used with caution. Clients CAN easily spoof IP addresses.
// Fetching the "real" client is done in a best-effort basis and can be [grossly inaccurate & precarious].
// You should especially heed this warning if you intend to use the IP addresses for security related activities.
// Proceed at your own risk.
//
// [grossly inaccurate & precarious]: https://adam-p.ca/blog/2022/03/x-forwarded-for/
func ClientIP(r *http.Request) string {
	return clientip.Get(r)
}

// clientIP is a middleware that adds the "real" client IP address to the request context.
// The IP can then be fetched using [ClientIP]
//
// Warning: This middleware should be used with care. Clients CAN easily spoof IP addresses.
// Fetching the "real" client is done in a best-effort basis and can be [grossly inaccurate & precarious].
//
// [grossly inaccurate & precarious]: https://adam-p.ca/blog/2022/03/x-forwarded-for/
func clientIP(wrappedHandler http.Handler, strategy ClientIPstrategy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var clientAddr string
		switch v := strategy; v {
		case DirectIpStrategy:
			clientAddr = clientip.DirectAddress(r.RemoteAddr)
		case LeftIpStrategy:
			clientAddr = clientip.Leftmost(r.Header)
		case RightIpStrategy:
			clientAddr = clientip.Rightmost(r.Header)
		case ProxyStrategy:
			clientAddr = clientip.ProxyHeader(r.Header)
		default:
			// treat everything else as a `singleIP` strategy
			clientAddr = clientip.SingleIPHeader(string(v), r.Header)
		}

		r = clientip.With(r, clientAddr)
		wrappedHandler.ServeHTTP(w, r)
	}
}
