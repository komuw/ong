package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/akshayjshah/attest"
)

func someClientIpHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, msg)
	}
}

func TestClientIP(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := clientIP(someClientIpHandler(msg))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := clientIP(someClientIpHandler(msg))

		runhandler := func() {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), msg)
		}

		wg := &sync.WaitGroup{}
		for rN := 0; rN <= 11; rN++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				runhandler()
			}()
		}
		wg.Wait()
	})
}

// TODO: rename.
func TestTodo(t *testing.T) {
	t.Parallel()

	t.Run("remoteAddrStrategy", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)

		ip := remoteAddrStrategy(req.RemoteAddr)
		attest.NotZero(t, ip)
		fmt.Println("ip: ", ip, " : ", req.RemoteAddr)
	})

	t.Run("singleIPHeaderStrategy", func(t *testing.T) {
		t.Run("privateIp", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			headerName := "Fly-Client-IP"
			hdrVal := "169.254.169.254" // AWS metadata api IP address.
			req.Header.Add(headerName, hdrVal)

			ip := singleIPHeaderStrategy(headerName, req.Header)
			attest.Zero(t, ip)
		})
		t.Run("not privateIp", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			headerName := "Fly-Client-IP"
			hdrVal := "93.184.216.34"
			req.Header.Add(headerName, hdrVal)

			ip := singleIPHeaderStrategy(headerName, req.Header)
			attest.NotZero(t, ip)
			attest.Equal(t, ip, hdrVal)
		})
		t.Run("bad header", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			headerName := xForwardedForHeader
			hdrVal := "93.184.216.34"
			req.Header.Add(headerName, hdrVal)

			ip := singleIPHeaderStrategy(headerName, req.Header)
			attest.Zero(t, ip)
		})
	})

	// t.Run("leftmostNonPrivateStrategy", func(t *testing.T) {
	// 	t.Run("privateIp", func(t *testing.T) {
	// 		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
	// 		headerName := "Fly-Client-IP"
	// 		hdrVal := "169.254.169.254" // AWS metadata api IP address.
	// 		req.Header.Add(headerName, hdrVal)

	// 		ip := leftmostNonPrivateStrategy(headerName, req.Header)
	// 		attest.Zero(t, ip)
	// 	})
	// 	t.Run("not privateIp", func(t *testing.T) {
	// 		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
	// 		headerName := "Fly-Client-IP"
	// 		hdrVal := "93.184.216.34"
	// 		req.Header.Add(headerName, hdrVal)

	// 		ip := leftmostNonPrivateStrategy(headerName, req.Header)
	// 		attest.NotZero(t, ip)
	// 		attest.Equal(t, ip, hdrVal)
	// 		fmt.Println("ip: ", ip, " : ", req.RemoteAddr)
	// 	})
	// })
}
