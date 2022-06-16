package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/akshayjshah/attest"
)

func someCorsHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, msg)
	}
}

func TestCorsPreflight(t *testing.T) {
	t.Parallel()

	t.Run("preflight success", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := Cors(someCorsHandler(msg), nil, nil, nil)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodOptions, "/someUri", nil)
		req.Header.Add(acrmHeader, "is-set") // preflight request header set
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusNoContent)
		attest.Equal(t, string(rb), "") // someCorsHandler is NOT called.
	})

	t.Run("http OPTIONS without preflight request header", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := Cors(someCorsHandler(msg), nil, nil, nil)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodOptions, "/someUri", nil)
		// preflight request header NOT set
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg) // someCorsHandler is called.
	})
}

// func TestCorsActualRequest(t *testing.T) {
// 	t.Parallel()

// 	t.Run("TODO", func(t *testing.T) {
// 		t.Parallel()

// 		msg := "hello"
// 		wrappedHandler := Cors(someCorsHandler(msg))
// 		rec := httptest.NewRecorder()
// 		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
// 		wrappedHandler.ServeHTTP(rec, req)

// 		res := rec.Result()
// 		defer res.Body.Close()

// 		rb, err := io.ReadAll(res.Body)
// 		attest.Ok(t, err)

// 		attest.Equal(t, res.StatusCode, http.StatusOK)
// 		attest.Equal(t, string(rb), msg)
// 	})
// }
