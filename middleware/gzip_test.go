package middleware

import (
	stdGzip "compress/gzip"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"go.akshayshah.org/attest"

	nytimes "github.com/NYTimes/gziphandler"
	klauspost "github.com/klauspost/compress/gzhttp"
	tmthrgd "github.com/tmthrgd/gziphandler"
)

func someGzipHandler(msg string) http.HandlerFunc {
	// bound stack growth.
	// see: https://github.com/komuw/ong/issues/54
	fMsg := strings.Repeat(msg, 3)
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, fMsg)
	}
}

func handlerImplementingFlush(msg string) http.HandlerFunc {
	iterations := 3
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

func login() http.HandlerFunc {
	tmpl, err := template.New("myTpl").Parse(`<!DOCTYPE html>
<html>

<body>
	<h2>Welcome to awesome website.</h2>
	<form method="POST">
	<label>Email:</label><br>
	<input type="text" id="email" name="email"><br>
	<label>First Name:</label><br>
	<input type="text" id="firstName" name="firstName"><br>

	<input type="hidden" id="{{.CsrfTokenName}}" name="{{.CsrfTokenName}}" value="{{.CsrfTokenValue}}"><br>
	<input type="submit">
	</form>

	<script nonce="{{.CspNonceValue}}">
	console.log("hello world");
	</script>

</body>

</html>`)
	if err != nil {
		panic(err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			data := struct {
				CsrfTokenName  string
				CsrfTokenValue string
				CspNonceValue  string
			}{
				CsrfTokenName:  CsrfTokenFormName,
				CsrfTokenValue: GetCsrfToken(r.Context()),
				CspNonceValue:  GetCspNonce(r.Context()),
			}
			if err = tmpl.Execute(w, data); err != nil {
				panic(err)
			}
			return
		}

		if err = r.ParseForm(); err != nil {
			panic(err)
		}

		_, _ = fmt.Fprintf(w, "you have submitted: %s", r.Form)
	}
}

func readBody(t *testing.T, res *http.Response) (strBody string) {
	t.Helper()

	defer res.Body.Close()
	body := res.Body

	if res.Header.Get(contentEncodingHeader) == "gzip" {
		// the body is gzipped.
		reader, err := stdGzip.NewReader(body)
		attest.Ok(t, err)
		defer reader.Close()
		rb, err := io.ReadAll(reader)
		attest.Ok(t, err)
		strBody = string(rb)
	} else {
		// body is gzipped
		rb, err := io.ReadAll(body)
		attest.Ok(t, err)
		strBody = string(rb)
	}

	return strBody
}

func TestGzip(t *testing.T) {
	t.Parallel()

	t.Run("http HEAD is not gzipped", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := gzip(someGzipHandler(msg))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodHead, "/someUri", nil)
		req.Header.Add(acceptEncodingHeader, "br;q=1.0, gzip;q=0.8, *;q=0.1")
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		strBody := readBody(t, res)

		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Subsequence(t, strBody, msg)
	})

	t.Run("middleware succeds", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := gzip(someGzipHandler(msg))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.Header.Add(acceptEncodingHeader, "br;q=1.0, gzip;q=0.8, *;q=0.1")
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		strBody := readBody(t, res)

		attest.Equal(t, res.Header.Get(contentEncodingHeader), "gzip")
		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Subsequence(t, strBody, msg)
	})

	t.Run("http.Flusher is supported and zipped", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := gzip(handlerImplementingFlush(msg))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.Header.Add(acceptEncodingHeader, "br;q=1.0, gzip;q=0.8, *;q=0.1")
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		strBody := readBody(t, res)

		attest.True(t, rec.Flushed)
		attest.Equal(t, res.Header.Get(contentEncodingHeader), "gzip")
		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Subsequence(t, strBody, msg)
		attest.Subsequence(t, strBody, "FlusherCalled")
	})

	t.Run("http.Flusher is supported and small is not zipped", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := gzip(handlerImplementingFlush(msg))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.Header.Add(acceptEncodingHeader, "br;q=1.0, gzip;q=0.8, *;q=0.1")
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		strBody := readBody(t, res)

		attest.True(t, rec.Flushed)
		attest.NotZero(t, res.Header.Get(contentEncodingHeader))
		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Subsequence(t, strBody, msg)
		attest.Subsequence(t, strBody, "FlusherCalled")
	})

	t.Run("without gzip acceptEncoding not zipped", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		wrappedHandler := gzip(someGzipHandler(msg))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.Header.Add(acceptEncodingHeader, "br;q=1.0, compress;q=0.8, *;q=0.1")
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		strBody := readBody(t, res)

		attest.Zero(t, res.Header.Get(contentEncodingHeader))
		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Subsequence(t, strBody, msg)
	})

	t.Run("issues/81", func(t *testing.T) {
		t.Parallel()

		wrappedHandler := gzip(login())

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.Header.Add(acceptEncodingHeader, "br;q=1.0, gzip;q=0.8, *;q=0.1")
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		strBody := readBody(t, res)

		attest.Equal(t, res.Header.Get(contentEncodingHeader), "gzip")
		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Subsequence(t, strBody, "Welcome to awesome website.")
	})

	t.Run("issues/81", func(t *testing.T) {
		t.Parallel()

		wrappedHandler := gzip(login())

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.Header.Add(acceptEncodingHeader, "br;q=1.0, gzip;q=0.8, *;q=0.1")
		wrappedHandler.ServeHTTP(rec, req)

		res := rec.Result()
		strBody := readBody(t, res)

		attest.Equal(t, res.Header.Get(contentEncodingHeader), "gzip")
		attest.Equal(t, res.StatusCode, http.StatusOK)
		attest.Subsequence(t, strBody, "Welcome to awesome website.")
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		msg := "hello"
		// for this concurrency test, we have to re-use the same wrappedHandler
		// so that state is shared and thus we can see if there is any state which is not handled correctly.
		wrappedHandler := gzip(someGzipHandler(msg))

		runhandler := func() {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			req.Header.Add(acceptEncodingHeader, "br;q=1.0, gzip;q=0.8, *;q=0.1")
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			strBody := readBody(t, res)

			attest.Equal(t, res.Header.Get(contentEncodingHeader), "gzip")
			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Subsequence(t, strBody, msg)
		}

		wg := &sync.WaitGroup{}
		for rN := 0; rN <= 10; rN++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				runhandler()
			}()
		}
		wg.Wait()
	})
}

//////////////////////////////////////////////////////////////////////// BENCHMARKS ////////////////////////////////////////////////////////////////////////
// note: Im not making any claims about which is faster or not.

/*
goos: linux
goarch: amd64
pkg: github.com/komuw/ong/middleware
cpu: Intel(R) Core(TM) i7-10510U CPU @ 1.80GHz

BenchmarkNoGzip-8          	      19	  56_144_904 ns/op	 3_038_712 B/op	      77 allocs/op
BenchmarkOngGzip-8       	      10	 102_784_222 ns/op	 4_408_756 B/op	     112 allocs/op
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

func BenchmarkOngGzip(b *testing.B) {
	var r int
	wrappedHandler := gzip(gzipBenchmarkHandler())

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
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
	for range b.N {
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
	for range b.N {
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
	for range b.N {
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
	wrappedHandler := gzip(gzipBenchmarkHandler())

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
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
