// TODO: doc comment
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

type sessionContextKeyType string

const sessCtxKey = sessionContextKeyType("ong-session-key")

// TODO: doc comment
func Get(r *http.Request, key string) string {
	ctx := r.Context()
	if vCtx := ctx.Value(sessCtxKey); vCtx != nil {
		if s, ok := vCtx.(map[string]string); ok {
			if val, ok := s[key]; ok {
				return val
			}
		}
	}

	return ""
}

// TODO: doc comment
func Set(r *http.Request, key, value string) {
	ctx := r.Context()
	if vCtx := ctx.Value(sessCtxKey); vCtx != nil {
		fmt.Println("aaaaaaaaa")
		if s, ok := vCtx.(map[string]string); ok {
			s[key] = value
			ctx = context.WithValue(ctx, sessCtxKey, s)
			r = r.WithContext(ctx)
		}
	}
}

// TODO: doc comment: sets multiple.
func SetM(r *http.Request, m M) {
	ctx := r.Context()
	if vCtx := ctx.Value(sessCtxKey); vCtx != nil {
		if s, ok := vCtx.(map[string]string); ok {
			maps.Copy(s, m)
			ctx = context.WithValue(ctx, sessCtxKey, s)
			r = r.WithContext(ctx)
		}
	}
}

// TODO: doc comment
type M map[string]string

// TODO: doc comment
// TODO: move to middleware/
// TODO: should this middleware should take some options(like cookie max-age) as arguments??
func Session(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	// TODO: make this variables
	cookieName := "ong_sess"
	secretKey := "secretKey"
	domain := "localhost"
	mAge := 2 * time.Hour

	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Read from cookies and check for session cookie.
		// 2. get that cookie and save it to r.context

		ctx := r.Context()
		c, err := cookie.GetEncrypted(r, cookieName, secretKey)
		if err == nil && c.Value != "" {
			v := map[string]string{}
			if err := json.Unmarshal([]byte(c.Value), &v); err == nil {
				ctx = context.WithValue(ctx, sessCtxKey, v)
				r = r.WithContext(ctx)
			}
		} else {
			v := map[string]string{}
			ctx = context.WithValue(ctx, sessCtxKey, v)
			r = r.WithContext(ctx)
		}

		defer func() {
			// 1. Save session cookie to response.
			ctx := r.Context()
			fmt.Println("4: ", ctx.Value(sessCtxKey))
			if vCtx := ctx.Value(sessCtxKey); vCtx != nil {
				if s, ok := vCtx.(map[string]string); ok {
					fmt.Println("save: s: ", s)
					if value, err := json.Marshal(s); err == nil && value != nil {
						fmt.Println("set cookie: string(value): ", string(value))
						cookie.SetEncrypted(r, w, cookieName, string(value), domain, mAge, secretKey)
					}
				}
			}
		}()

		wrappedHandler(w, r)
	}
}
