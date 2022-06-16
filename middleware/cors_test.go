package middleware

import (
	"fmt"
	"testing"
)

func TestCreateWildcards(t *testing.T) {
	t.Run("TODO", func(t *testing.T) {
		allowedOrigins = []string{"hello.com", "hi", "http://*.example.com"}
		createWildcards()
		fmt.Println("\n\t allowedWildcardOrigins: ", allowedWildcardOrigins)

		// req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		// isOriginAllowed(req, origin string)
	})
}
