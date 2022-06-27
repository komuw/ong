package middleware

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/akshayjshah/attest"

	nytimes "github.com/NYTimes/gziphandler"
	klauspost "github.com/klauspost/compress/gzhttp"
	tmthrgd "github.com/tmthrgd/gziphandler"
)

func someGzipHandler(msg string, iterations int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if iterations > (3 * defaultMinSize) {
			// bound stack growth.
			// see: https://github.com/komuw/goweb/issues/54
			iterations = 3 * defaultMinSize
		}
		msg = strings.Repeat(msg, iterations)
		fmt.Fprint(w, msg)
	}
}

func handlerImplementingFlush(msg string, iterations int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if f, ok := w.(http.Flusher); ok {
			msg = "FlusherCalled::" + strings.Repeat(msg, iterations)
			fmt.Fprint(w, msg)

			f.Flush()
		} else {
			msg = strings.Repeat(msg, iterations)
			fmt.Fprint(w, msg)
		}
	}
}

func TestGzip(t *testing.T) {
	t.Parallel()

	t.Run("http HEAD is not gzipped", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := Gzip(someGzipHandler(msg, 1))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodHead, "/someUri", nil)
		req.Header.Add(acceptEncodingHeader, "br;q=1.0, gzip;q=0.8, *;q=0.1")
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)
	})

	t.Run("small responses are not gzipped", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		iterations := (defaultMinSize / 100)
		wrappedHandler := Gzip(someGzipHandler(msg, iterations))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.Header.Add(acceptEncodingHeader, "br;q=1.0, gzip;q=0.8, *;q=0.1")
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), msg)
		attest.Zero(t, res.Header.Get(contentEncodingHeader))
	})

	t.Run("middleware succeds", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		iterations := defaultMinSize * 2
		wrappedHandler := Gzip(someGzipHandler(msg, iterations))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.Header.Add(acceptEncodingHeader, "br;q=1.0, gzip;q=0.8, *;q=0.1")
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		reader, err := gzip.NewReader(res.Body)
		attest.Ok(t, err)
		defer reader.Close()

		rb, err := io.ReadAll(reader)
		attest.Ok(t, err)

		attest.Equal(t, res.Header.Get(contentEncodingHeader), "gzip")
		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.True(t, strings.Contains(string(rb), msg))
	})

	t.Run("http.Flusher is supported and zipped", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		iterations := defaultMinSize * 2
		wrappedHandler := Gzip(handlerImplementingFlush(msg, iterations))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.Header.Add(acceptEncodingHeader, "br;q=1.0, gzip;q=0.8, *;q=0.1")
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		reader, err := gzip.NewReader(res.Body)
		attest.Ok(t, err)
		defer reader.Close()

		rb, err := io.ReadAll(reader)
		attest.Ok(t, err)

		attest.True(t, rec.Flushed)
		attest.Equal(t, res.Header.Get(contentEncodingHeader), "gzip")
		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.True(t, strings.Contains(string(rb), msg))
		attest.True(t, strings.Contains(string(rb), "FlusherCalled"))
	})

	t.Run("http.Flusher is supported and small is not zipped", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		iterations := 1
		wrappedHandler := Gzip(handlerImplementingFlush(msg, iterations))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.Header.Add(acceptEncodingHeader, "br;q=1.0, gzip;q=0.8, *;q=0.1")
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.True(t, rec.Flushed)
		attest.Zero(t, res.Header.Get(contentEncodingHeader))
		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.True(t, strings.Contains(string(rb), msg))
		attest.True(t, strings.Contains(string(rb), "FlusherCalled"))
	})

	t.Run("without gzip acceptEncoding not zipped", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		iterations := defaultMinSize * 2
		wrappedHandler := Gzip(someGzipHandler(msg, iterations))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.Header.Add(acceptEncodingHeader, "br;q=1.0, compress;q=0.8, *;q=0.1")
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		rb, err := io.ReadAll(res.Body)
		attest.Ok(t, err)

		attest.Zero(t, res.Header.Get(contentEncodingHeader))
		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Equal(t, string(rb), strings.Repeat(msg, iterations))
	})
}

//////////////////////////////////////////////////////////////////////// BENCHMARKS ////////////////////////////////////////////////////////////////////////
// note: Im not making any claims about which is faster or not.

/*
goos: linux
goarch: amd64
pkg: github.com/komuw/goweb/middleware
cpu: Intel(R) Core(TM) i7-10510U CPU @ 1.80GHz

BenchmarkNoGzip-8          	      19	  56_144_904 ns/op	 3_038_712 B/op	      77 allocs/op
BenchmarkGoWebGzip-8       	      10	 102_784_222 ns/op	 4_408_756 B/op	     112 allocs/op
BenchmarkKlauspostGzip-8   	       7	 149_572_590 ns/op	 3_327_585 B/op	     106 allocs/op
BenchmarkNytimesGzip-8     	       4	 315_386_476 ns/op	 3_813_934 B/op	     116 allocs/op
BenchmarkTmthrgdGzip-8     	       4	 319_786_254 ns/op	 3_527_012 B/op	     116 allocs/op
*/

func gzipBenchmarkHandler() http.HandlerFunc {
	bin, err := os.ReadFile("testdata/benchmark.json")
	if err != nil {
		panic(err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, bin)
	}
}

var result int //nolint:gochecknoglobals

func BenchmarkGoWebGzip(b *testing.B) {
	var r int
	wrappedHandler := Gzip(gzipBenchmarkHandler())

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		// always record the result of Fib to prevent
		// the compiler eliminating the function call.
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.Header.Add(acceptEncodingHeader, "br;q=1.0, gzip;q=0.8, *;q=0.1")
		wrappedHandler.ServeHTTP(rec, req)
		res := rec.Result()
		defer res.Body.Close()

		attest.Equal(b, res.Header.Get(contentEncodingHeader), "gzip")
		attest.Equal(b, res.StatusCode, http.StatusOK)
		r = res.StatusCode
	}
	// always store the result to a package level variable
	// so the compiler cannot eliminate the Benchmark itself.
	result = r
}

func BenchmarkKlauspostGzip(b *testing.B) {
	var r int
	wrappedHandler := klauspost.GzipHandler(gzipBenchmarkHandler())

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		// always record the result of Fib to prevent
		// the compiler eliminating the function call.
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.Header.Add(acceptEncodingHeader, "br;q=1.0, gzip;q=0.8, *;q=0.1")
		wrappedHandler.ServeHTTP(rec, req)
		res := rec.Result()
		defer res.Body.Close()

		attest.Equal(b, res.Header.Get(contentEncodingHeader), "gzip")
		attest.Equal(b, res.StatusCode, http.StatusOK)
		r = res.StatusCode
	}
	// always store the result to a package level variable
	// so the compiler cannot eliminate the Benchmark itself.
	result = r
}

func BenchmarkNytimesGzip(b *testing.B) {
	var r int
	wrappedHandler := nytimes.GzipHandler(gzipBenchmarkHandler())

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		// always record the result of Fib to prevent
		// the compiler eliminating the function call.
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.Header.Add(acceptEncodingHeader, "br;q=1.0, gzip;q=0.8, *;q=0.1")
		wrappedHandler.ServeHTTP(rec, req)
		res := rec.Result()
		defer res.Body.Close()

		attest.Equal(b, res.Header.Get(contentEncodingHeader), "gzip")
		attest.Equal(b, res.StatusCode, http.StatusOK)
		r = res.StatusCode
	}
	// always store the result to a package level variable
	// so the compiler cannot eliminate the Benchmark itself.
	result = r
}

func BenchmarkTmthrgdGzip(b *testing.B) {
	var r int
	wrappedHandler := tmthrgd.Gzip(gzipBenchmarkHandler())

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		// always record the result of Fib to prevent
		// the compiler eliminating the function call.
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.Header.Add(acceptEncodingHeader, "br;q=1.0, gzip;q=0.8, *;q=0.1")
		wrappedHandler.ServeHTTP(rec, req)
		res := rec.Result()
		defer res.Body.Close()

		attest.Equal(b, res.Header.Get(contentEncodingHeader), "gzip")
		attest.Equal(b, res.StatusCode, http.StatusOK)
		r = res.StatusCode
	}
	// always store the result to a package level variable
	// so the compiler cannot eliminate the Benchmark itself.
	result = r
}

func BenchmarkNoGzip(b *testing.B) {
	var r int
	wrappedHandler := Gzip(gzipBenchmarkHandler())

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		// always record the result of Fib to prevent
		// the compiler eliminating the function call.
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.Header.Add(acceptEncodingHeader, "br;q=1.0, deflate;q=0.8, *;q=0.1")
		wrappedHandler.ServeHTTP(rec, req)
		res := rec.Result()
		defer res.Body.Close()

		attest.Equal(b, res.Header.Get(contentEncodingHeader), "")
		attest.Equal(b, res.StatusCode, http.StatusOK)
		r = res.StatusCode
	}
	// always store the result to a package level variable
	// so the compiler cannot eliminate the Benchmark itself.
	result = r
}

//////////////////////////////////////////////////////////////////////// BENCHMARKS ////////////////////////////////////////////////////////////////////////
