package middleware

import (
	"log"
	"net/http"
	"runtime/debug"
)

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
