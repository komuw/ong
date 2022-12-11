package middleware

import (
	"net/http"

	"github.com/komuw/ong/internal/clientip"
)

// Most of the code here is insipired by(or taken from):
//
//	(a) https://github.com/realclientip/realclientip-go whose license(BSD Zero Clause License) can be found here: https://github.com/realclientip/realclientip-go/blob/v1.0.0/LICENSE
//

type (
	clientIPstrategy string
)

const (
	// DirectIpStrategy is a middleware option that describes the strategy to use when fetching the client's IP address.
	// This strategy should be used if the server accepts direct connections, rather than through a proxy.
	//
	// See the warning in [GetClientIP]
	DirectIpStrategy = clientIPstrategy("DirectIpStrategy")

	// LeftIpStrategy is a middleware option that describes the strategy to use when fetching the client's IP address.
	// It derives the client IP from the leftmost valid and non-private IP address in the `X-Fowarded-For` or `Forwarded` header.
	// Note: This MUST NOT be used for security purposes. This IP can be trivially SPOOFED.
	//
	// See the warning in [GetClientIP]
	LeftIpStrategy = clientIPstrategy("LeftIpStrategy")

	// RightIpStrategy is a middleware option that describes the strategy to use when fetching the client's IP address.
	// It derives the client IP from the rightmost valid and non-private IP address in the `X-Fowarded-For` or `Forwarded` header.
	RightIpStrategy = clientIPstrategy("RightIpStrategy")
)

// SingleIpStrategy is a middleware option that describes the strategy to use when fetching the client's IP address.
// It derives the client IP from http header headerName.
//
// headerName MUST not be either `X-Forwarded-For` or `Forwarded`.
// It can be something like `CF-Connecting-IP`, `Fastly-Client-IP`, `Fly-Client-IP`, etc; depending on your usecase.
//
// See the warning in [GetClientIP]
func SingleIpStrategy(headerName string) clientIPstrategy {
	return clientIPstrategy(headerName)
}

// clientIP is a middleware that adds the "real" client IP address to the request context.
// The IP can then be fetched using [GetClientIP]
//
// Warning: This middleware should be used with care. Clients CAN easily spoof IP addresses.
// Fetching the "real" client is done in a best-effort basis and can be [grossly inaccurate & precarious].
//
// [grossly inaccurate & precarious]: https://adam-p.ca/blog/2022/03/x-forwarded-for/
func clientIP(wrappedHandler http.HandlerFunc, strategy clientIPstrategy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var clientAddr string
		switch v := strategy; v {
		case DirectIpStrategy:
			clientAddr = clientip.DirectAddrStrategy(r.RemoteAddr)
		case LeftIpStrategy:
			clientAddr = clientip.LeftmostNonPrivateStrategy(r.Header)
		case RightIpStrategy:
			clientAddr = clientip.RightmostNonPrivateStrategy(r.Header)
		default:
			// treat everything else as a `singleIP` strategy
			clientAddr = clientip.SingleIPHeaderStrategy(string(v), r.Header)
		}

		r = clientip.WithClientIP(r, clientAddr)
		wrappedHandler(w, r)
	}
}
