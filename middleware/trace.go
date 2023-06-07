package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/komuw/ong/cookie"
	"github.com/komuw/ong/internal/octx"
	"github.com/komuw/ong/log"
)

const logIDKey = string(octx.LogCtxKey)

// trace is a middleware that adds logID to request and response.
func trace(wrappedHandler http.Handler, domain string) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// set cookie/headers/ctx for logID.
			logID := getLogId(r)
			ctx = context.WithValue(
				ctx,
				// using this custom key is important, instead of using `logIDKey`
				octx.LogCtxKey,
				logID,
			)
			r = r.WithContext(ctx)
			r.Header.Set(logIDKey, logID)
			w.Header().Set(logIDKey, logID)
			cookie.Set(
				w,
				logIDKey,
				logID,
				domain,
				// Hopefully 15mins is enough.
				// Google considers a session to be 30mins.
				// https://support.google.com/analytics/answer/2731565?hl=en#time-based-expiration
				15*time.Minute,
				false,
			)

			wrappedHandler.ServeHTTP(w, r)
		},
	)
}

// getLogId returns a logID from the request or autogenerated if not available from the request.
func getLogId(req *http.Request) string {
	fromHeader := func(r *http.Request) string {
		if r != nil {
			if hdr := r.Header.Get(logIDKey); hdr != "" {
				return hdr
			}
		}
		return ""
	}

	fromCookie := func(r *http.Request) string {
		if r != nil {
			var cookies []*http.Cookie
			for _, v := range r.Cookies() {
				if v.Name == logIDKey && v.Value != "" {
					cookies = append(cookies, v)
				}
			}
			// there can be multiple cookies with the same name. get the latest
			if len(cookies) > 0 {
				return cookies[len(cookies)-1].Value
			}
		}
		return ""
	}

	fromCtx := func(ctx context.Context) string {
		return log.GetId(ctx) // we want a unique id, here.
	}

	// get logid in order of preference;
	// - http headers
	// - cookie
	// - context.Context

	if logID := fromHeader(req); logID != "" {
		return logID
	}

	if logID := fromCookie(req); logID != "" {
		return logID
	}

	return fromCtx(req.Context())
}
