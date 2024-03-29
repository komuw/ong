package middleware

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/komuw/ong/errors"
	"github.com/komuw/ong/log"
)

// Some of the code here is inspired(or taken from) by:
//   (a) https://github.com/eliben/code-for-blog whose license(Unlicense) can be found here: https://github.com/eliben/code-for-blog/blob/464a32f686d7646ba3fc612c19dbb550ec8a05b1/LICENSE

// recoverer is a middleware that recovers from panics in wrappedHandler.
// When/if a panic occurs, it logs the stack trace and returns an InternalServerError response.
func recoverer(wrappedHandler http.Handler, l *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			errR := recover()
			if errR != nil {
				reqL := log.WithID(r.Context(), l)

				code := http.StatusInternalServerError
				status := http.StatusText(code)

				msg := "http_server"
				flds := []any{
					"error", fmt.Sprint(errR),
					"clientIP", ClientIP(r),
					"clientFingerPrint", ClientFingerPrint(r),
					"method", r.Method,
					"path", r.URL.Redacted(),
					"code", code,
					"status", status,
				}
				if ongError := w.Header().Get(ongMiddlewareErrorHeader); ongError != "" {
					extra := []any{"ongError", ongError}
					flds = append(flds, extra...)
				}
				w.Header().Del(ongMiddlewareErrorHeader) // remove header so that users dont see it.

				if e, ok := errR.(error); ok {
					extra := []any{"err", errors.Wrap(e)} // wrap with ong/errors so that the log will have a stacktrace.
					flds = append(flds, extra...)
					reqL.Error(msg, flds...)
				} else {
					reqL.Error(msg, flds...)
				}

				// respond.
				http.Error(
					w,
					status,
					code,
				)
			}
		}()

		wrappedHandler.ServeHTTP(w, r)
	}
}
