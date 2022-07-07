package middleware

import (
	"fmt"
	"net/http"

	"github.com/komuw/ong/log"
)

// Most of the code here is insipired(or taken from) by:
//   (a) https://github.com/eliben/code-for-blog whose license(Unlicense) can be found here: https://github.com/eliben/code-for-blog/blob/464a32f686d7646ba3fc612c19dbb550ec8a05b1/LICENSE

// Panic is a middleware that recovers from panics in wrappedHandler.
// It logs the stack trace and returns an InternalServerError response.
func Panic(wrappedHandler http.HandlerFunc, l log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			err := recover()
			if err != nil {
				reqL := l.WithCtx(r.Context()).WithCaller()

				code := http.StatusInternalServerError
				status := http.StatusText(code)
				http.Error(
					w,
					status,
					code,
				)

				flds := log.F{
					"err":         fmt.Sprint(err),
					"requestAddr": r.RemoteAddr,
					"method":      r.Method,
					"path":        r.URL.EscapedPath(),
					"code":        code,
					"status":      status,
				}
				if ongError := w.Header().Get(ongMiddlewareErrorHeader); ongError != "" {
					flds["ongError"] = ongError
				}
				w.Header().Del(ongMiddlewareErrorHeader) // remove header so that users dont see it.

				if e, ok := err.(error); ok {
					reqL.Error(e, flds)
				} else {
					reqL.Error(nil, flds)
				}
			}
		}()

		wrappedHandler(w, r)
	}
}
