// Package sess provides an implementation of http sessions that is backed by tamper-proof & encrypted cookies.
// This package should ideally be used together with the [middleware.Session] middleware.
package sess

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/komuw/ong/cookie"
	"golang.org/x/exp/maps"
)

type (
	sessionContextKeyType string
	// M is an alias of map[string]string
	M = map[string]string
)

const (
	ctxKey = sessionContextKeyType("ong-session-key")
	// CookieName is the name of the http cookie under which sessions are stored.
	CookieName = "ong_sess"
)

// Initialise returns a new http.Request (based on r) that has sessions properly setup.
//
// You do not need to call this function, if you are also using the [middleware.Session] middleware.
// That middleware does so automatically for you.
func Initialise(r *http.Request, secretKey string) *http.Request {
	ctx := r.Context()
	var sessVal M // should be per request.

	c, err := cookie.GetEncrypted(r, CookieName, secretKey)
	if err == nil && c.Value != "" {
		if errM := json.Unmarshal([]byte(c.Value), &sessVal); errM == nil {
			ctx = context.WithValue(ctx, ctxKey, sessVal)
			r = r.WithContext(ctx)
		}
	}

	if sessVal == nil {
		// The process above might have failed; maybe `json.Unmarshal` failed.
		sessVal = M{}
		ctx = context.WithValue(ctx, ctxKey, sessVal)
		r = r.WithContext(ctx)
	}

	return r
}

// Set adds the key-value pair to the current http session.
func Set(r *http.Request, key, value string) {
	ctx := r.Context()
	if vCtx := ctx.Value(ctxKey); vCtx != nil {
		if s, ok := vCtx.(M); ok {
			s[key] = value
			ctx = context.WithValue(ctx, ctxKey, s)
			r = r.WithContext(ctx)
		}
	} else {
		s := M{key: value}
		ctx = context.WithValue(ctx, ctxKey, s)
		r = r.WithContext(ctx)
	}
}

// SetM adds multiple key-value pairs to the current http session.
func SetM(r *http.Request, m M) {
	ctx := r.Context()
	if vCtx := ctx.Value(ctxKey); vCtx != nil {
		if s, ok := vCtx.(M); ok {
			maps.Copy(s, m)
			ctx = context.WithValue(ctx, ctxKey, s)
			r = r.WithContext(ctx)
		}
	} else {
		ctx = context.WithValue(ctx, ctxKey, m)
		r = r.WithContext(ctx)
	}
}

// Get retrieves the value corresponding to the given key from the current http session.
// It returns an empty string if key is not found in the session.
func Get(r *http.Request, key string) string {
	ctx := r.Context()
	if vCtx := ctx.Value(ctxKey); vCtx != nil {
		if s, ok := vCtx.(M); ok {
			if val, okM := s[key]; okM {
				return val
			}
		}
	} else {
		s := M{}
		ctx = context.WithValue(ctx, ctxKey, s)
		r = r.WithContext(ctx)
	}

	return ""
}

// GetM retrieves all the key-value pairs found from the current http session.
// It returns nil if none is found.
func GetM(r *http.Request) M {
	ctx := r.Context()
	if vCtx := ctx.Value(ctxKey); vCtx != nil {
		if s, ok := vCtx.(M); ok {
			return s
		}
	} else {
		s := M{}
		ctx = context.WithValue(ctx, ctxKey, s)
		r = r.WithContext(ctx)
	}

	return nil
}

// Save writes(to http cookies) any key-value pairs that have already been added to the current http session.
//
// You do not need to call this function, if you are also using the [middleware.Session] middleware.
// That middleware does so automatically for you.
func Save(
	r *http.Request,
	w http.ResponseWriter,
	domain string,
	mAge time.Duration,
	secretKey string,
) {
	ctx := r.Context()
	if vCtx := ctx.Value(ctxKey); vCtx != nil {
		if s, ok := vCtx.(M); ok {
			if value, err := json.Marshal(s); err == nil && value != nil {
				cookie.SetEncrypted(
					r,
					w,
					CookieName,
					string(value),
					domain,
					mAge,
					secretKey,
				)
			}
		}
	} else {
		s := M{}
		ctx = context.WithValue(ctx, ctxKey, s)
		r = r.WithContext(ctx)
	}
}
