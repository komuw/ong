package middleware

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
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
		wrappedHandler := loadShedder(someLoadShedderHandler(msg))

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
		req.Header.Set(loadShedderTestHeader, "5")
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
		// for this concurrency test, we have to re-use the same wrappedHandler
		// so that state is shared and thus we can see if there is any state which is not handled correctly.
		wrappedHandler := loadShedder(someLoadShedderHandler(msg))

		runhandler := func() {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			req.Header.Set(loadShedderTestHeader, "4")
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), msg)
		}

		wg := &sync.WaitGroup{}
		for rN := 0; rN <= 50+minSampleSize; rN++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				runhandler()
			}()
		}
		wg.Wait()
	})
}

func TestPercentile(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		{
			lq := latencyQueue{
				sl: []time.Duration{
					5 * time.Second,
					6 * time.Second,
					7 * time.Second,
					8 * time.Second,
					9 * time.Second,
					0 * time.Second,
					1 * time.Second,
					2 * time.Second,
					3 * time.Second,
					4 * time.Second,
				},
			}

			got := percentile(lq.sl, 25, len(lq.sl))
			attest.Equal(t, got, 2000*time.Millisecond) // ie, 2seconds
		}
		{
			lq := latencyQueue{}
			for i := 1; i <= 1000; i++ {
				lq.sl = append(
					lq.sl,
					time.Duration(i)*time.Second,
				)
			}
			got := percentile(lq.sl, 99, len(lq.sl))
			attest.Equal(t, got.Seconds(), 991)
		}
	})
}

func TestLatencyQueue(t *testing.T) {
	t.Parallel()

	t.Run("all samples taken within samplingPeriod", func(t *testing.T) {
		t.Parallel()

		minSampleSize := 10
		lq := latencyQueue{}
		for i := 1; i <= 1000; i++ {
			lq.sl = append(
				lq.sl,
				time.Duration(i)*time.Second,
			)
		}

		got := lq.getP99(minSampleSize)
		attest.Equal(t, got.Seconds(), 991)
	})

	t.Run("number of samples less than minSampleSize", func(t *testing.T) {
		t.Parallel()

		minSampleSize := 10_000
		lq := latencyQueue{}
		for i := 1; i <= (minSampleSize / 2); i++ {
			lq.sl = append(
				lq.sl,
				time.Duration(i)*time.Second,
			)
		}

		got := lq.getP99(minSampleSize)
		attest.Zero(t, got)
	})

	t.Run("issues/217: order is preserved", func(t *testing.T) {
		t.Parallel()

		// See: https://github.com/komuw/ong/issues/217

		lq := newLatencyQueue()
		for i := 1; i <= maxLatItems; i++ {
			lq.sl = append(
				lq.sl,
				time.Duration(i)*time.Second,
			)
		}

		// add very small latency to be latest in the queue.
		smallLatency := 1 * time.Nanosecond
		for i := 1; i <= 20; i++ {
			lq.sl = append(lq.sl, smallLatency)
		}

		// call percentile which may mutate the latency slice.
		_ = percentile(lq.sl, 90, len(lq.sl))

		// resize.
		lq.reSize()

		latest := lq.sl[len(lq.sl)-1]
		secondLatest := lq.sl[len(lq.sl)-1]
		attest.Equal(t, latest, smallLatency)
		attest.Equal(t, secondLatest, smallLatency)
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		lq := newLatencyQueue()

		wg := &sync.WaitGroup{}
		for rN := 0; rN <= 50+minSampleSize; rN++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				lq.add(1 * time.Second)
				lq.reSize()
				lq.getP99(3)
			}()
		}
		wg.Wait()
	})
}

func loadShedderBenchmarkHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		latency := time.Duration(rand.Intn(100)+1) * time.Millisecond
		time.Sleep(latency)
		fmt.Fprint(w, "hey")
	}
}

func BenchmarkLoadShedder(b *testing.B) {
	var r int

	wrappedHandler := loadShedder(loadShedderBenchmarkHandler())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/someUri", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < 100; n++ {
		wrappedHandler.ServeHTTP(rec, req)
		res := rec.Result()
		defer res.Body.Close()
		attest.Equal(b, res.StatusCode, http.StatusOK)
		r = res.StatusCode
	}
	// always store the result to a package level variable
	// so the compiler cannot eliminate the Benchmark itself.
	result = r
}
