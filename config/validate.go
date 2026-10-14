package config

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/komuw/ong/errors"
)

// allowedOrigins are used by both the cors and csrf middlewares.
const originsErrPrefix = "ong/middleware/cors,csrf: "

func validateAllowedOrigins(allowedOrigins []string) error {
	/*
		origin is defined by the scheme (protocol), hostname (domain), and port
		https://developer.mozilla.org/en-US/docs/Glossary/Origin
	*/
	if len(allowedOrigins) > 1 && slices.Contains(allowedOrigins, "*") {
		return errors.New(originsErrPrefix + "single wildcard should not be used together with other allowedOrigins")
	}

	if len(allowedOrigins) == 1 && allowedOrigins[0] == "*" {
		return nil
	}

	for _, origin := range allowedOrigins {
		u, err := url.Parse(origin)
		if err != nil {
			return fmt.Errorf(originsErrPrefix+"%w", err)
		}

		if u.Scheme == "" {
			return fmt.Errorf(originsErrPrefix+"scheme should not be empty `%v`", origin)
		}
		if u.Host == "" {
			return fmt.Errorf(originsErrPrefix+"host should not be empty `%v`", origin)
		}
		if u.Path != "" {
			return fmt.Errorf(originsErrPrefix+"should not contain url path `%v`", origin)
		}
		if !strings.EqualFold(origin, u.Scheme+"://"+u.Host) {
			// Browsers send origins without userinfo, query or fragment.
			return fmt.Errorf(originsErrPrefix+"should only contain scheme, host and optional port `%v`", origin)
		}

		if strings.Contains(origin, "*") {
			// The wildcard "*" can only be used on its own, ie allowedOrigins == []string{"*"}.
			// Wildcard subdomains like `https://*.example.com` are not supported. They are hard to match correctly;
			// eg `https://*example.com` would also match `https://attackerexample.com`.
			// They would also need to be trusted by the csrf middleware, which only supports exact origins.
			return fmt.Errorf(originsErrPrefix+"wildcard should only be used on its own `%v`", origin)
		}
	}

	return nil
}

func validateAllowedMethods(allowedMethods []string) error {
	// There are some methods that are disallowed.
	// https://fetch.spec.whatwg.org/#methods
	for _, m := range allowedMethods {
		if slices.Contains([]string{"CONNECT", "TRACE", "TRACK"}, strings.ToUpper(m)) {
			return fmt.Errorf("ong/middleware/cors: method is forbidden in CORS allowedMethods `%v`", m)
		}
	}

	return nil
}

func validateAllowedRequestHeaders(allowedHeaders []string) error {
	// There are some headers that are disallowed.
	// https://fetch.spec.whatwg.org/#terminology-headers
	for _, h := range allowedHeaders {
		if slices.Contains([]string{
			"ACCEPT-CHARSET",
			"ACCEPT-ENCODING",
			"ACCESS-CONTROL-REQUEST-HEADERS",
			"ACCESS-CONTROL-REQUEST-METHOD",
			"CONNECTION",
			"CONTENT-LENGTH",
			"COOKIE",
			"COOKIE2",
			"DATE",
			"DNT",
			"EXPECT",
			"HOST",
			"KEEP-ALIVE",
			"ORIGIN",
			"REFERER",
			"SET-COOKIE",
			"TE",
			"TRAILER",
			"TRANSFER-ENCODING",
			"Upgrade",
			"VIA",
		},
			// Spec says; "If name is a byte-case-insensitive match"
			// So the use of `strings.ToUpper` here is correct.
			strings.ToUpper(h),
		) {
			return fmt.Errorf("ong/middleware/cors: header is forbidden in CORS allowedHeaders `%v`", h)
		}

		if strings.HasPrefix(strings.ToLower(h), "proxy-") || strings.HasPrefix(strings.ToLower(h), "sec-") {
			// Spec says; "If name when byte-lowercased starts with `proxy-` or `sec-`"
			// So the use of `strings.ToLower` here is correct.
			return fmt.Errorf("ong/middleware/cors: header is forbidden in CORS allowedHeaders `%v`", h)
		}
	}

	return nil
}

func validateAllowCredentials(
	allowCredentials bool,
	allowedOrigins []string,
	allowedMethods []string,
	allowedHeaders []string,
) error {
	// Credentialed requests cannot be used with wildcard.
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS#credentialed_requests_and_wildcards

	// `validateAllowedOrigins` has already checked that wildcard can only exist in slice of len 1.
	if allowCredentials && len(allowedOrigins) == 1 && allowedOrigins[0] == "*" {
		return errors.New("ong/middleware/cors: allowCredentials should not be used together with wildcard allowedOrigins")
	}
	if allowCredentials && len(allowedMethods) == 1 && allowedMethods[0] == "*" {
		return errors.New("ong/middleware/cors: allowCredentials should not be used together with wildcard allowedMethods")
	}
	if allowCredentials && len(allowedHeaders) == 1 && allowedHeaders[0] == "*" {
		return errors.New("ong/middleware/cors: allowCredentials should not be used together with wildcard allowedHeaders")
	}

	if allowCredentials {
		// Credentialed requests should not be used with 'http' scheme. Should require 'https'.
		// https://jub0bs.com/posts/2023-02-08-fearless-cors/#disallow-insecure-origins-by-default
		// https://portswigger.net/research/exploiting-cors-misconfigurations-for-bitcoins-and-bounties
		for _, origin := range allowedOrigins {
			u, err := url.Parse(origin)
			if err != nil {
				return fmt.Errorf("ong/middleware/cors: %w", err)
			}
			if u.Scheme == "http" {
				return fmt.Errorf("ong/middleware/cors: allowCredentials should not be used together with origin that uses unsecure scheme `%v`", origin)
			}
		}
	}

	return nil
}

func validateLoadShedPercentile(loadShedPercentile float64) error {
	// This also rejects NaN, since all comparisons with NaN are false.
	if !(loadShedPercentile >= 0 && loadShedPercentile <= 100) {
		return fmt.Errorf("ong/middleware/loadshed: loadShedPercentile should be in the range 0 to 100 `%v`", loadShedPercentile)
	}

	return nil
}
