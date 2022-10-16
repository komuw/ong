// TODO: doc comment
// TODO: comment that they are backed by cookies(encrypted)
// TODO: mention importance of using session middleware.
package sess

import (
	"context"
	"encoding/json"
	"fmt"
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
	// TODO: doc comment
	CtxKey = sessionContextKeyType("ong-session-key")
	// TODO: doc comment
	CookieName = "ong_sess"
)

// TODO: doc comment
// TODO: remind people they don't need to call it if they are also using [middleware.Session]
func Initialise(r *http.Request, secretKey string) *http.Request {
	ctx := r.Context()
	var sessVal map[string]string // should be per request.

	c, err := cookie.GetEncrypted(r, CookieName, secretKey)
	if err == nil && c.Value != "" {
		if err := json.Unmarshal([]byte(c.Value), &sessVal); err == nil {
			ctx = context.WithValue(ctx, CtxKey, sessVal)
			r = r.WithContext(ctx)
		}
	}

	if sessVal == nil {
		// The process above might have failed; maybe `json.Unmarshal` failed.
		sessVal = map[string]string{}
		ctx = context.WithValue(ctx, CtxKey, sessVal)
		r = r.WithContext(ctx)
	}

	return r
}

// TODO: doc comment
func Set(r *http.Request, key, value string) {
	ctx := r.Context()
	if vCtx := ctx.Value(CtxKey); vCtx != nil {
		if s, ok := vCtx.(map[string]string); ok {
			s[key] = value
			ctx = context.WithValue(ctx, CtxKey, s)
			r = r.WithContext(ctx)
		}
	} else {
		s := map[string]string{key: value}
		ctx = context.WithValue(ctx, CtxKey, s)
		r = r.WithContext(ctx)
	}
}

// TODO: doc comment: sets multiple.
func SetM(r *http.Request, m M) {
	ctx := r.Context()
	if vCtx := ctx.Value(CtxKey); vCtx != nil {
		if s, ok := vCtx.(map[string]string); ok {
			maps.Copy(s, m)
			ctx = context.WithValue(ctx, CtxKey, s)
			r = r.WithContext(ctx)
		}
	} else {
		ctx = context.WithValue(ctx, CtxKey, m)
		r = r.WithContext(ctx)
	}
}

// TODO: doc comment
func Get(r *http.Request, key string) string {
	ctx := r.Context()
	if vCtx := ctx.Value(CtxKey); vCtx != nil {
		if s, ok := vCtx.(map[string]string); ok {
			if val, ok := s[key]; ok {
				return val
			}
		}
	} else {
		s := map[string]string{}
		ctx = context.WithValue(ctx, CtxKey, s)
		r = r.WithContext(ctx)
	}

	return ""
}

// TODO: doc comment. gets all/multiple.
func GetM(r *http.Request) map[string]string {
	ctx := r.Context()
	if vCtx := ctx.Value(CtxKey); vCtx != nil {
		if s, ok := vCtx.(map[string]string); ok {
			return s
		}
	} else {
		s := map[string]string{}
		ctx = context.WithValue(ctx, CtxKey, s)
		r = r.WithContext(ctx)
	}

	return nil
}

// TODO: doc comment
// TODO: remind people they don't need to call it if they are also using [middleware.Session]
func Save(
	r *http.Request,
	w http.ResponseWriter,
	domain string,
	mAge time.Duration,
	secretKey string,
) {
	ctx := r.Context()
	if vCtx := ctx.Value(CtxKey); vCtx != nil {
		if s, ok := vCtx.(map[string]string); ok {
			if value, err := json.Marshal(s); err == nil && value != nil {
				fmt.Println("set cookie: string(value): ", string(value))
				cookie.SetEncrypted(r, w, CookieName, string(value), domain, mAge, secretKey)
			}
		}
	} else {
		s := map[string]string{}
		ctx = context.WithValue(ctx, CtxKey, s)
		r = r.WithContext(ctx)
	}
}
