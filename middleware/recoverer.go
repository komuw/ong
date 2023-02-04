package middleware

import (
	"fmt"
	"net/http"
	"os"

	"github.com/komuw/ong/errors"
	"golang.org/x/exp/slog"
)

// Most of the code here is insipired(or taken from) by:
//   (a) https://github.com/eliben/code-for-blog whose license(Unlicense) can be found here: https://github.com/eliben/code-for-blog/blob/464a32f686d7646ba3fc612c19dbb550ec8a05b1/LICENSE

// recoverer is a middleware that recovers from panics in wrappedHandler.
// When/if a panic occurs, it logs the stack trace and returns an InternalServerError response.
func recoverer(wrappedHandler http.HandlerFunc, l *slog.Logger) http.HandlerFunc {
	pid := os.Getpid()

	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			errR := recover()
			if errR != nil {
				reqL := l.WithContext(r.Context())

				code := http.StatusInternalServerError
				status := http.StatusText(code)
				http.Error(
					w,
					status,
					code,
				)

				msg := "http_server" // TODO: (komuw) check if this okay
				// TODO: (komuw) check if this okay
				flds := []any{
					"error", fmt.Sprint(errR),
					"clientIP", ClientIP(r),
					"method", r.Method,
					"path", r.URL.Redacted(),
					"code", code,
					"status", status,
					"pid", pid,
				}
				if ongError := w.Header().Get(ongMiddlewareErrorHeader); ongError != "" {
					extra := []any{"ongError", ongError}
					flds = append(flds, extra)
				}
				w.Header().Del(ongMiddlewareErrorHeader) // remove header so that users dont see it.

				if e, ok := errR.(error); ok {
					reqL.Error(msg, errors.Wrap(e), flds...) // wrap with ong/errors so that the log will have a stacktrace.
				} else {
					reqL.Error(msg, nil, flds...)
				}
			}
		}()

		wrappedHandler(w, r)
	}
}
