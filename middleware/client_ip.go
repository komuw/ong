package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// Most of the code here is insipired by(or taken from):
//
//	(a) https://github.com/realclientip/realclientip-go whose license(BSD Zero Clause License) can be found here: https://github.com/realclientip/realclientip-go/blob/v1.0.0/LICENSE
//

/*
Algorithm:
==========
1. Collect all of the IPs:
   Make a single list of all the IPs in all of the X-Forwarded-For headers. Also have the RemoteAddr available.
2. Decide your security needs:
   Default to using the rightmost-ish approach. Only use the leftmost-ish if you have to, and make sure you do so carefully.
3. Leftmost-ish:
   Closest to the "real IP", but utterly untrustworthy
   If your server is directly connected to the internet, there might be an XFF header or there might not be (depending on whether the client used a proxy).
   If there is an XFF header, pick the leftmost IP address that is a valid, non-private IPv4 or IPv6 address.
   If there is no XFF header, use the RemoteAddr.
   If your server is behind one or more reverse proxies, pick the leftmost XFF IP address that is a valid, non-private IPv4 or IPv6 address. (If there’s no XFF header, you need to fix your network configuration problem right now.)
   And never forget the security implications!
4. Rightmost-ish:
   The only useful IP you can trust
   If your server is directly connected to the internet, the XFF header cannot be trusted, period. Use the RemoteAddr.
   There are more details here...

- https://adam-p.ca/blog/2022/03/x-forwarded-for/#algorithms
*/

type (
	clientIPstrategy       string
	clientIPcontextKeyType string
)

const (
	errPrefix           = "ong/middleware:"
	xForwardedForHeader = "X-Forwarded-For"
	forwardedHeader     = "Forwarded"

	// clientIPctxKey is the name of the context key used to store the client IP address.
	clientIPctxKey = clientIPcontextKeyType("clientIPcontextKeyType")

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

// ClientIP returns the "real" client IP address.
//
// Warning: This should be used with care. Clients CAN easily spoof IP addresses.
// Fetching the "real" client is done in a best-effort basis and can be [grossly inaccurate & precarious].
// You should especially heed this warning if you intend to use the IP addresses for security related activities.
// Proceed at your own peril.
//
// [grossly inaccurate & precarious]: https://adam-p.ca/blog/2022/03/x-forwarded-for/
func ClientIP(r *http.Request) string {
	if vCtx := r.Context().Value(clientIPctxKey); vCtx != nil {
		if s, ok := vCtx.(string); ok {
			return s
		}
	}

	ip, _, _ := net.SplitHostPort(r.RemoteAddr) // ignore error.
	return ip
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
		ctx := r.Context()

		var clientAddr string
		switch v := strategy; v {
		case DirectIpStrategy:
			clientAddr = directAddrStrategy(r.RemoteAddr)
		case LeftIpStrategy:
			clientAddr = leftmostNonPrivateStrategy(xForwardedForHeader, r.Header)
			if clientAddr == "" {
				clientAddr = leftmostNonPrivateStrategy(forwardedHeader, r.Header)
			}
		case RightIpStrategy:
			clientAddr = rightmostNonPrivateStrategy(xForwardedForHeader, r.Header)
			if clientAddr == "" {
				clientAddr = rightmostNonPrivateStrategy(forwardedHeader, r.Header)
			}
		default:
			// treat everything else as a `singleIP` strategy
			clientAddr = singleIPHeaderStrategy(string(v), r.Header)
		}

		ctx = context.WithValue(ctx, clientIPctxKey, clientAddr)
		r = r.WithContext(ctx)

		wrappedHandler(w, r)
	}
}

// directAddrStrategy returns the client socket IP, stripped of port.
// This strategy should be used if the server accepts direct connections, rather than through a proxy.
func directAddrStrategy(remoteAddr string) string {
	if ipAddr := goodIPAddr(remoteAddr); ipAddr != nil {
		return ipAddr.String()
	}

	return ""
}

// singleIPHeaderStrategy derives an IP address from a single-IP header.
// A non-exhaustive list of such single-IP headers is:
// X-Real-IP, CF-Connecting-IP, True-Client-IP, Fastly-Client-IP, X-Azure-ClientIP, X-Azure-SocketIP, Fly-Client-IP.
// This strategy should be used when the given header is added by a trusted reverse proxy.
// You MUST ensure that this header is not spoofable (as is possible with Akamai's use of
// True-Client-IP, Fastly's default use of Fastly-Client-IP, and Azure's X-Azure-ClientIP).
// See the single-IP wiki page for more info: https://github.com/realclientip/realclientip-go/wiki/Single-IP-Headers
//
// The returned IP may contain a zone identifier.
// If no valid IP can be derived, empty string will be returned.
func singleIPHeaderStrategy(headerName string, headers http.Header) string {
	headerName = http.CanonicalHeaderKey(headerName)

	if headerName == xForwardedForHeader || headerName == forwardedHeader {
		// This is because those headers are actually list of values.
		return ""
	}

	// RFC 2616 does not allow multiple instances of single-IP headers (or any non-list header).
	// It is debatable whether it is better to treat multiple such headers as an error
	// (more correct) or simply pick one of them (more flexible). As we've already
	// told the user tom make sure the header is not spoofable, we're going to use the
	// last header instance if there are multiple. (Using the last is arbitrary, but
	// in theory it should be the newest value.)
	ipStr := lastHeader(headers, headerName)
	if ipStr == "" {
		// There is no header
		return ""
	}

	ipAddr := goodIPAddr(ipStr)
	if ipAddr == nil {
		// The header value is invalid
		return ""
	}

	return ipAddr.String()
}

// leftmostNonPrivateStrategy derives the client IP from the leftmost valid and
// non-private IP address in the X-Fowarded-For or Forwarded header.
// This strategy should be used when a valid, non-private IP closest to the client is desired.
// Note: This MUST NOT be used for security purposes. This IP can be trivially SPOOFED.
//
// The returned IP may contain a zone identifier.
// If no valid IP can be derived, empty string will be returned.
func leftmostNonPrivateStrategy(headerName string, headers http.Header) string {
	headerName = http.CanonicalHeaderKey(headerName)

	if headerName != xForwardedForHeader && headerName != forwardedHeader {
		// This is because the header we expect here is one that is a list of values(which xForwardedForHeader and forwardedHeader are.)
		return ""
	}

	ipAddrs := getIPAddrList(headers, headerName)
	for _, ip := range ipAddrs {
		if isSafeIp(ip) {
			// This is the leftmost valid, non-private IP
			return ip.String()
		}
	}

	// We failed to find any valid, non-private IP
	return ""
}

// rightmostNonPrivateStrategy derives the client IP from the rightmost valid and
// non-private IP address in the X-Fowarded-For or Forwarded header.
// This strategy should be used when all reverse proxies between the internet and the server have private-space IP addresses.
//
// The returned IP may contain a zone identifier.
// If no valid IP can be derived, empty string will be returned.
func rightmostNonPrivateStrategy(headerName string, headers http.Header) string {
	headerName = http.CanonicalHeaderKey(headerName)

	if headerName != xForwardedForHeader && headerName != forwardedHeader {
		// This is because the header we expect here is one that is a list of values(which xForwardedForHeader and forwardedHeader are.)
		return ""
	}

	ipAddrs := getIPAddrList(headers, headerName)
	// Look backwards through the list of IP addresses
	for i := len(ipAddrs) - 1; i >= 0; i-- {
		ip := ipAddrs[i]
		if isSafeIp(ip) {
			// This is the rightmost non-private IP
			return ip.String()
		}
	}

	// We failed to find any valid, non-private IP
	return ""
}

// goodIPAddr parses IP address and adds a check for unspecified (like "::") and zero-value addresses (like "0.0.0.0").
// These are nominally valid IPs (net.ParseIP will accept them), but they are undesirable for the purposes of this library.
func goodIPAddr(ipStr string) *netip.Addr {
	host, _, err := net.SplitHostPort(ipStr)
	if err == nil {
		// `SplitHostPort` may error with something like `missing port in address`
		// We continue neverthless since `netip.ParseAddr` below will also do validation.
		ipStr = host
	}

	ipAddr, err := netip.ParseAddr(ipStr)
	if err != nil {
		return nil
	}

	theIp := &ipAddr
	if !isSafeIp(theIp) {
		return nil
	}

	return theIp
}

func isSafeIp(addr *netip.Addr) bool {
	if addr == nil {
		return false
	}
	if addr.IsUnspecified() {
		return false
	}
	if addr.IsLoopback() {
		return false
	}
	if addr.IsLinkLocalUnicast() {
		return false
	}
	if addr.IsPrivate() {
		return false
	}

	return addr.IsValid()
}

// trimMatchedEnds trims s if and only if the first and last bytes in s are in chars.
// If chars is a single character (like `"`), then the first and last bytes MUST match that single character.
// If chars is two characters (like `[]`), the first byte in s MUST match the first byte in chars,
// and the last bytes in s MUST match the last byte in chars.
// This helps us ensure that we only trim _matched_ quotes and brackets, which strings.Trim doesn't provide.
func trimMatchedEnds(s, chars string) (string, error) {
	if len(chars) != 1 && len(chars) != 2 {
		return "", fmt.Errorf("%s %s", errPrefix, "trimMatchedEnds chars must be length 1 or 2")
	}

	first, last := chars[0], chars[0]
	if len(chars) > 1 {
		last = chars[1]
	}

	if len(s) < 2 {
		return s, nil
	}

	if s[0] != first {
		return s, nil
	}

	if s[len(s)-1] != last {
		return s, nil
	}

	return s[1 : len(s)-1], nil
}

// lastHeader returns the last header with the given name.
// It returns empty string if the header is not found or if the header has an empty value.
// No validation is done on the IP string. headerName MUST already be canonicalized.
// This should be used with single-IP headers, like X-Real-IP.
// Per RFC 2616, they should not have multiple headers, but if they do we can hope we're getting the newest/best by taking the last instance.
// This MUST NOT be used with list headers, like X-Forwarded-For and Forwarded.
func lastHeader(headers http.Header, headerName string) string {
	// Note that Go's Header map uses canonicalized keys
	matches, ok := headers[headerName]
	if !ok || len(matches) == 0 {
		// For our uses of this function, returning an empty string in this case is fine
		return ""
	}

	return matches[len(matches)-1]
}

// getIPAddrList creates a single list of all of the X-Forwarded-For or Forwarded header values, in order.
// Any invalid IPs will result in nil elements. headerName MUST already be canonicalized.
func getIPAddrList(headers http.Header, headerName string) (result []*netip.Addr) {
	// There may be multiple XFF headers present. We need to iterate through them all,
	// in order, and collect all of the IPs.
	// Note that we're not joining all of the headers into a single string and then
	// splitting. Doing it that way would use more memory.
	// Note that Go's Header map uses canonicalized keys.
	for _, h := range headers[headerName] {
		// We now have a string with comma-separated list items
		for _, rawListItem := range strings.Split(h, ",") {
			// The IPs are often comma-space separated, so we'll need to trim the string
			rawListItem = strings.TrimSpace(rawListItem)

			var ipAddr *netip.Addr
			// If this is the XFF header, rawListItem is just an IP;
			// if it's the Forwarded header, then there's more parsing to do.
			if headerName == xForwardedForHeader {
				ipAddr = goodIPAddr(rawListItem)
			} else {
				ipAddr = parseForwardedListItem(rawListItem)
			}

			// ipAddr is nil if not valid
			result = append(result, ipAddr)
		}
	}

	// Possible performance improvements:
	// Here we are parsing _all_ of the IPs in the XFF headers, but we don't need all of
	// them. Instead, we could start from the left or the right (depending on strategy),
	// parse as we go, and stop when we've come to the one we want. But that would make
	// the various strategies somewhat more complex.

	return result
}

// parseForwardedListItem parses a Forwarded header list item, and returns the "for" IP address.
// It returns nil if the "for" IP is absent or invalid.
func parseForwardedListItem(fwd string) *netip.Addr {
	// The header list item can look like these kinds of thing:
	//	For="[2001:db8:cafe::17%zone]:4711"
	//	For="[2001:db8:cafe::17%zone]"
	//	for=192.0.2.60;proto=http; by=203.0.113.43
	//	for=192.0.2.43

	// First split up "for=", "by=", "host=", etc.
	fwdParts := strings.Split(fwd, ";")

	// Find the "for=" part, since that has the IP we want (maybe)
	var forPart string
	for _, fp := range fwdParts {
		// Whitespace is allowed around the semicolons
		fp = strings.TrimSpace(fp)

		fpSplit := strings.Split(fp, "=")
		if len(fpSplit) != 2 {
			// There are too many or too few equal signs in this part
			continue
		}

		if strings.EqualFold(fpSplit[0], "for") {
			// We found the "for=" part
			forPart = fpSplit[1]
			break
		}
	}

	// There shouldn't (per RFC 7239) be spaces around the semicolon or equal sign. It might
	// be more correct to consider spaces an error, but we'll tolerate and trim them.
	forPart = strings.TrimSpace(forPart)

	// Get rid of any quotes, such as surrounding IPv6 addresses.
	// Note that doing this without checking if the quotes are present means that we are
	// effectively accepting IPv6 addresses that don't strictly conform to RFC 7239, which
	// requires quotes. https://www.rfc-editor.org/rfc/rfc7239#section-4
	// This behaviour is debatable.
	// It also means that we will accept IPv4 addresses with quotes, which is correct.
	forPart, err := trimMatchedEnds(forPart, `"`)
	if err != nil {
		return nil
	}
	if forPart == "" {
		// We failed to find a "for=" part
		return nil
	}

	ipAddr := goodIPAddr(forPart)
	if ipAddr == nil {
		// The IP extracted from the "for=" part isn't valid
		return nil
	}

	return ipAddr
}
