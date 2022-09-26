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
		wrappedHandler := LoadShedder(someLoadShedderHandler(msg))

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
		wrappedHandler := LoadShedder(someLoadShedderHandler(msg))

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
			got := percentile(lq.sl, 25)
			attest.Equal(t, got, 2250*time.Millisecond) // ie, 2.25seconds
		}
		{
			lq := latencyQueue{}
			for i := 1; i <= 1000; i++ {
				lq.sl = append(
					lq.sl,
					time.Duration(i)*time.Second,
				)
			}
			got := percentile(lq.sl, 99)
			attest.Equal(t, got.Seconds(), 990.01)
		}
	})
}

func TestLatencyQueue(t *testing.T) {
	t.Parallel()

	t.Run("all samples taken outside samplingPeriod", func(t *testing.T) {
		t.Parallel()

		now := time.Now().UTC()
		samplingPeriod := 10 * time.Millisecond
		minSampleSize := 10

		lq := latencyQueue{}
		for i := 1; i <= 1000; i++ {
			lq.sl = append(
				lq.sl,
				time.Duration(i)*time.Second,
			)
		}

		got := lq.getP99(now, samplingPeriod, minSampleSize)
		attest.Zero(t, got)
	})

	t.Run("all samples taken within samplingPeriod", func(t *testing.T) {
		t.Parallel()

		now := time.Now().UTC()
		samplingPeriod := 10000 * time.Millisecond
		minSampleSize := 10

		lq := latencyQueue{}
		for i := 1; i <= 1000; i++ {
			lq.sl = append(
				lq.sl,
				time.Duration(i)*time.Second,
			)
		}

		got := lq.getP99(now, samplingPeriod, minSampleSize)
		attest.Equal(t, got.Seconds(), 990.01)
	})

	t.Run("number of samples less than minSampleSize", func(t *testing.T) {
		t.Parallel()

		now := time.Now().UTC()
		samplingPeriod := 10000 * time.Millisecond
		minSampleSize := 10_000

		lq := latencyQueue{}
		for i := 1; i <= (minSampleSize / 2); i++ {
			lq.sl = append(
				lq.sl,
				time.Duration(i)*time.Second,
			)
		}

		got := lq.getP99(now, samplingPeriod, minSampleSize)
		attest.Zero(t, got)
	})

	t.Run("all samples taken in the future", func(t *testing.T) {
		t.Parallel()

		now := time.Now().UTC()
		samplingPeriod := 10000 * time.Millisecond
		minSampleSize := 10

		lq := latencyQueue{}
		for i := 1; i <= 1000; i++ {
			lq.sl = append(
				lq.sl,
				time.Duration(i)*time.Second,
			)
		}

		got := lq.getP99(now, samplingPeriod, minSampleSize)
		attest.Zero(t, got)
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
				lq.getP99(time.Now().UTC(), 1*time.Second, 3)
			}()
		}
		wg.Wait()
	})
}

func loadShedderBenchmarkHandler() http.HandlerFunc {
	rand.Seed(time.Now().UTC().UnixNano())
	return func(w http.ResponseWriter, r *http.Request) {
		latency := time.Duration(rand.Intn(100)+1) * time.Millisecond
		time.Sleep(latency)
		fmt.Fprint(w, "hey")
	}
}

func BenchmarkLoadShedder(b *testing.B) {
	var r int

	wrappedHandler := LoadShedder(loadShedderBenchmarkHandler())
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
