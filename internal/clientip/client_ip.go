// Package clientip provides(in a best effort manner) a client's IP address.
package clientip

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// Most of the code here is inspired by(or taken from):
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

// ClientIPstrategy is an option that describes the strategy to use when fetching the client's IP address.
type ClientIPstrategy string

type clientIPcontextKeyType string

const (
	errPrefix           = "ong/internal/clientip:"
	xForwardedForHeader = "X-Forwarded-For"
	forwardedHeader     = "Forwarded"
	proxyHeader         = "PROXY"
	// clientIPctxKey is the name of the context key used to store the client IP address.
	clientIPctxKey = clientIPcontextKeyType("clientIPcontextKeyType")
)

// Get returns the "real" client IP address.
// See [github.com/komuw/ong/middleware.ClientIP]
func Get(r *http.Request) string {
	if vCtx := r.Context().Value(clientIPctxKey); vCtx != nil {
		if s, ok := vCtx.(string); ok && s != "" {
			return s
		}
	}

	ip, _, _ := net.SplitHostPort(r.RemoteAddr) // ignore error.
	return ip
}

// With returns a [*http.Request] whose context contains a client IP address.
func With(r *http.Request, clientAddr string) *http.Request {
	ctx := r.Context()
	ctx = context.WithValue(ctx, clientIPctxKey, clientAddr)
	r = r.WithContext(ctx)

	return r
}

// DirectAddress returns the client socket IP, stripped of port.
// This strategy should be used if the server accepts direct connections, rather than through a proxy.
func DirectAddress(remoteAddr string) string {
	if ipAddr := goodIPAddr(remoteAddr); ipAddr != nil {
		return ipAddr.String()
	}

	return ""
}

// SingleIPHeader derives an IP address from a single-IP header.
// A non-exhaustive list of such single-IP headers is:
// X-Real-IP, CF-Connecting-IP, True-Client-IP, Fastly-Client-IP, X-Azure-ClientIP, X-Azure-SocketIP, Fly-Client-IP.
// This strategy should be used when the given header is added by a trusted reverse proxy.
// You MUST ensure that this header is not spoofable (as is possible with Akamai's use of
// True-Client-IP, Fastly's default use of Fastly-Client-IP, and Azure's X-Azure-ClientIP).
// See the [single-IP wiki] page for more info.
//
// The returned IP may contain a zone identifier.
// If no valid IP can be derived, empty string will be returned.
//
// [single-IP wiki]: https://github.com/realclientip/realclientip-go/wiki/Single-IP-Headers
func SingleIPHeader(headerName string, headers http.Header) string {
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

// Leftmost derives the client IP from the leftmost valid and
// non-private IP address in the X-Fowarded-For or Forwarded header.
// This strategy should be used when a valid, non-private IP closest to the client is desired.
// Note: This MUST NOT be used for security purposes. This IP can be trivially SPOOFED.
//
// The returned IP may contain a zone identifier.
// If no valid IP can be derived, empty string will be returned.
func Leftmost(headers http.Header) string {
	var theIP string

	{
		headerName := xForwardedForHeader // ought to be canonical. ie, http.CanonicalHeaderKey(xForwardedForHeader)
		ipAddrs := getIPAddrList(headers, headerName)
		for _, ip := range ipAddrs {
			if isSafeIp(ip) {
				// This is the leftmost valid, non-private IP
				theIP = ip.String()
				break
			}
		}
	}

	if theIP == "" {
		headerName := forwardedHeader
		ipAddrs := getIPAddrList(headers, headerName)
		for _, ip := range ipAddrs {
			if isSafeIp(ip) {
				// This is the leftmost valid, non-private IP
				theIP = ip.String()
				break
			}
		}
	}

	// We failed to find any valid, non-private IP
	return theIP
}

// Rightmost derives the client IP from the rightmost valid and
// non-private IP address in the X-Fowarded-For or Forwarded header.
// This strategy should be used when all reverse proxies between the internet and the server have private-space IP addresses.
//
// The returned IP may contain a zone identifier.
// If no valid IP can be derived, empty string will be returned.
func Rightmost(headers http.Header) string {
	var theIP string

	{
		headerName := xForwardedForHeader // ought to be canonical. ie, http.CanonicalHeaderKey(xForwardedForHeader)
		ipAddrs := getIPAddrList(headers, headerName)
		// Look backwards through the list of IP addresses
		for i := len(ipAddrs) - 1; i >= 0; i-- {
			ip := ipAddrs[i]
			if isSafeIp(ip) {
				// This is the rightmost non-private IP
				theIP = ip.String()
				break
			}
		}
	}

	if theIP == "" {
		headerName := forwardedHeader
		ipAddrs := getIPAddrList(headers, headerName)
		// Look backwards through the list of IP addresses
		for i := len(ipAddrs) - 1; i >= 0; i-- {
			ip := ipAddrs[i]
			if isSafeIp(ip) {
				// This is the rightmost non-private IP
				theIP = ip.String()
				break
			}
		}
	}

	// We failed to find any valid, non-private IP
	return theIP
}

// ProxyHeader derives an IP address based off the [PROXY protocol v1].
//
// [PROXY protocol v1]: https://www.haproxy.org/download/2.8/doc/proxy-protocol.txt
func ProxyHeader(headers http.Header) string {
	s := headers.Get(proxyHeader)

	if len(s) <= 15 {
		// The maximum line lengths of proxy-protocol line including the CRLF are:
		// ipv4: 56 chars, ipv6: 104 chars, unknown: 15 chars.
		// Thus any string of length less-than-or-equal-to 15 does not contain a valid client_ip.
		//
		// see: https://www.haproxy.org/download/2.8/doc/proxy-protocol.txt
		return ""
	}

	// The proxy protocol line is a single line that ends with a carriage return and line feed ("\r\n"), and has the following form:
	/*
		  PROXY_STRING +
		  single space +
		  INET_PROTOCOL +
		  single space +
		  CLIENT_IP +
		  single space +
		  PROXY_IP +
		  single space +
		  CLIENT_PORT +
		  single space +
		  PROXY_PORT +
		  "\r\n"

		eg:
		  PROXY TCP4 198.51.100.22 203.0.113.7 35646 80\r\n
		  PROXY TCP6 2001:DB8::21f:5bff:febf:ce22:8a2e 2001:DB8::12f:8baa:eafc:ce29:6b2e 35646 80\r\n

		- https://docs.aws.amazon.com/elasticloadbalancing/latest/classic/enable-proxy-protocol.html
	*/

	s1 := s[11:] // len("PROXY TCP6 ")
	ipStr := strings.Split(s1, " ")[0]

	ipAddr := goodIPAddr(ipStr)
	if ipAddr == nil {
		// The header value is invalid
		return ""
	}

	return ipAddr.String()
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
