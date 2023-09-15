package middleware

import (
	"net/http"

	"github.com/komuw/ong/config"
	"github.com/komuw/ong/internal/clientip"
)

// Some of the code here is inspired by(or taken from):
//
//	(a) https://github.com/realclientip/realclientip-go whose license(BSD Zero Clause License) can be found here: https://github.com/realclientip/realclientip-go/blob/v1.0.0/LICENSE
//

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
func clientIP(wrappedHandler http.Handler, strategy config.ClientIPstrategy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var clientAddr string
		switch v := strategy; v {
		case config.DirectIpStrategy:
			clientAddr = clientip.DirectAddress(r.RemoteAddr)
		case config.LeftIpStrategy:
			clientAddr = clientip.Leftmost(r.Header)
		case config.RightIpStrategy:
			clientAddr = clientip.Rightmost(r.Header)
		case config.ProxyStrategy:
			clientAddr = clientip.ProxyHeader(r.Header)
		default:
			// treat everything else as a `singleIP` strategy
			clientAddr = clientip.SingleIPHeader(string(v), r.Header)
		}

		r = clientip.With(r, clientAddr)
		wrappedHandler.ServeHTTP(w, r)
	}
}
