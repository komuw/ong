package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/akshayjshah/attest"
)

const loadShedderTestHeader = "LoadShedderTestHeader"

func someLoadShedderHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lat := r.Header.Get(loadShedderTestHeader)
		latency, err := strconv.Atoi(lat)
		if err != nil {
			panic(err)
		}
		time.Sleep(time.Duration(latency) * time.Millisecond)
		fmt.Fprint(w, msg)
	}
}

func TestLoadShedder(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := LoadShedder(someLoadShedderHandler(msg))

		for i := 0; i < 100; i++ {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			req.Header.Set(loadShedderTestHeader, fmt.Sprint(i))
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			// fmt.Println("\t i: ", i)
			fmt.Println("res.StatusCode: ", res.StatusCode)
			fmt.Println("gowebMiddlewareErrorHeader: ", res.Header.Get(gowebMiddlewareErrorHeader))
			fmt.Println("retryAfterHeader: ", res.Header.Get(retryAfterHeader))
			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), msg)
		}
	})

	// t.Run("success", func(t *testing.T) {
	// 	t.Parallel()

	// 	msg := "hello"
	// 	wrappedHandler := LoadShedder(someLoadShedderHandler(msg))

	// 	rec := httptest.NewRecorder()
	// 	req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
	// 	wrappedHandler.ServeHTTP(rec, req)

	// 	res := rec.Result()
	// 	defer res.Body.Close()

	// 	rb, err := io.ReadAll(res.Body)
	// 	attest.Ok(t, err)

	// 	attest.Equal(t, res.StatusCode, http.StatusOK)
	// 	attest.Equal(t, string(rb), msg)
	// })

	// t.Run("bad remoteAddr", func(t *testing.T) {
	// 	t.Parallel()

	// 	msg := "hello"
	// 	wrappedHandler := LoadShedder(someLoadShedderHandler(msg))

	// 	rec := httptest.NewRecorder()
	// 	req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
	// 	req.RemoteAddr = "BadRemoteAddr"
	// 	wrappedHandler.ServeHTTP(rec, req)

	// 	res := rec.Result()
	// 	defer res.Body.Close()

	// 	rb, err := io.ReadAll(res.Body)
	// 	attest.Ok(t, err)

	// 	attest.Equal(t, res.StatusCode, http.StatusOK)
	// 	attest.Equal(t, string(rb), msg)
	// })
}

func TestPercentile(t *testing.T) {
	t.Run("todo", func(t *testing.T) {
		lq := latencyQueue{
			latency{duration: 5 * time.Second},
			latency{duration: 6 * time.Second},
			latency{duration: 7 * time.Second},
			latency{duration: 8 * time.Second},
			latency{duration: 9 * time.Second},
			latency{duration: 0 * time.Second},
			latency{duration: 1 * time.Second},
			latency{duration: 2 * time.Second},
			latency{duration: 3 * time.Second},
			latency{duration: 4 * time.Second},
		}
		got := percentile(lq, 25)
		attest.Equal(t, got, 2250*time.Millisecond) // ie, 2.25seconds
	})
}
