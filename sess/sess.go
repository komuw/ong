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

// TODO: doc comment. gets all/multiple.
func GetM(r *http.Request) map[string]string {
	ctx := r.Context()
	if vCtx := ctx.Value(sessCtxKey); vCtx != nil {
		if s, ok := vCtx.(map[string]string); ok {
			return s
		}
	}

	return nil
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
		var sessVal map[string]string // should be per request.

		c, err := cookie.GetEncrypted(r, cookieName, secretKey)
		if err == nil && c.Value != "" {
			if err := json.Unmarshal([]byte(c.Value), &sessVal); err == nil {
				ctx = context.WithValue(ctx, sessCtxKey, sessVal)
				r = r.WithContext(ctx)
			}
		}

		if sessVal == nil {
			// The process above might have failed; maybe `json.Unmarshal` failed.
			sessVal = map[string]string{}
			ctx = context.WithValue(ctx, sessCtxKey, sessVal)
			r = r.WithContext(ctx)
		}

		// 1. Save session cookie to response.
		defer Save(r, w, domain, mAge, secretKey)

		wrappedHandler(w, r)
	}
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
	cookieName := "ong_sess"

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
}
