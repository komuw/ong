package middleware

import (
	"log"
	"net/http"
	"runtime/debug"
)

// Most of the code here is insipired by:
//   (a) https://github.com/eliben/code-for-blog license(Unlicense) can be found here: https://github.com/eliben/code-for-blog/blob/464a32f686d7646ba3fc612c19dbb550ec8a05b1/LICENSE

// Panic is a middleware that recovers from panics in wrappedHandler.
// It logs the stack trace and returns an InternalServerError response.
func Panic(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			err := recover()
			if err != nil {
				http.Error(
					w,
					http.StatusText(http.StatusInternalServerError),
					http.StatusInternalServerError,
				)

				// TODO: pass in a logger to this middleware.
				log.Println(string(debug.Stack()))
			}
		}()

		wrappedHandler(w, r)
	}
}
