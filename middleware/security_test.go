package middleware

import (
	"fmt"
	"net/http"
	"testing"
	// "github.com/akshayjshah/attest"
)

// echoHandler echos back in the response, the msg that was passed in.
func echoHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, msg)
	}
}

func TestSecurity(t *testing.T) {
	t.Parallel()

	t.Run("TODO", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		host := "example.com"
		got := Security(echoHandler(msg), host)
		_ = got
		// attest.Equal(t, got, "want")
	})
}
