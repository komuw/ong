package middleware

import (
	"fmt"
	"net/http"
	"os"

	"github.com/komuw/ong/errors"
	"github.com/komuw/ong/internal/clientip"
	"github.com/komuw/ong/log"
)

// Most of the code here is insipired(or taken from) by:
//   (a) https://github.com/eliben/code-for-blog whose license(Unlicense) can be found here: https://github.com/eliben/code-for-blog/blob/464a32f686d7646ba3fc612c19dbb550ec8a05b1/LICENSE

// recoverer is a middleware that recovers from panics in wrappedHandler.
// When/if a panic occurs, it logs the stack trace and returns an InternalServerError response.
func recoverer(wrappedHandler http.HandlerFunc, l log.Logger) http.HandlerFunc {
	pid := os.Getpid()

	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			errR := recover()
			if errR != nil {
				reqL := l.WithCtx(r.Context()).WithCaller()

				code := http.StatusInternalServerError
				status := http.StatusText(code)
				http.Error(
					w,
					status,
					code,
				)

				flds := log.F{
					"err":      fmt.Sprint(errR),
					"clientIP": clientip.Get(r),
					"method":   r.Method,
					"path":     r.URL.Redacted(),
					"code":     code,
					"status":   status,
					"pid":      pid,
				}
				if ongError := w.Header().Get(ongMiddlewareErrorHeader); ongError != "" {
					flds["ongError"] = ongError
				}
				w.Header().Del(ongMiddlewareErrorHeader) // remove header so that users dont see it.

				if e, ok := errR.(error); ok {
					reqL.Error(errors.Wrap(e), flds) // wrap with ong/errors so that the log will have a stacktrace.
				} else {
					reqL.Error(nil, flds)
				}
			}
		}()

		wrappedHandler(w, r)
	}
}
