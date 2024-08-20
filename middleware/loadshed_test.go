package middleware

import (
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/komuw/ong/config"
	"go.akshayshah.org/attest"
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
		wrappedHandler := loadShedder(
			someLoadShedderHandler(msg),
			config.DefaultLoadShedSamplingPeriod,
			config.DefaultLoadShedMinSampleSize,
			config.DefaultLoadShedBreachLatency,
			99,
		)

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
		wrappedHandler := loadShedder(
			someLoadShedderHandler(msg),
			config.DefaultLoadShedSamplingPeriod,
			config.DefaultLoadShedMinSampleSize,
			config.DefaultLoadShedBreachLatency,
			99,
		)

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
		for rN := 0; rN <= 50+config.DefaultLoadShedMinSampleSize; rN++ {
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
			lq := newLatencyQueue()
			for _, dur := range []time.Duration{
				5 * time.Second,
				0 * time.Second,
				6 * time.Second,
				8 * time.Second,
				9 * time.Second,
				1 * time.Second,
				2 * time.Second,
				7 * time.Second,
				3 * time.Second,
				4 * time.Second,
			} {
				lq.add(dur)
			}

			got := percentile(lq.sl, 25, len(lq.sl))
			attest.Equal(t, got, 2000) // ie, 2000millisecond(2seconds)
		}
		{
			lq := latencyQueue{}
			for i := 1; i <= 1000; i++ {
				dur := time.Duration(i) * time.Second
				lq.add(dur)
			}
			got := percentile(lq.sl, 99, len(lq.sl))
			gotM := time.Duration(got) * time.Millisecond
			attest.Equal(t, gotM.Seconds(), 991)
		}
		{
			lq := latencyQueue{}
			for i := 1000; i >= 1; i-- {
				dur := time.Duration(i) * time.Second
				lq.add(dur)
			}
			got := percentile(lq.sl, 99, len(lq.sl))
			gotM := time.Duration(got) * time.Millisecond
			attest.Equal(t, gotM.Seconds(), 991)
		}

		{ // different duration units mixed in.
			lq := newLatencyQueue()
			for _, dur := range []time.Duration{
				3 * time.Minute,
				1 * time.Second,
				4 * time.Microsecond,
				6 * time.Millisecond,
			} {
				lq.add(dur)
			}
			{
				got := percentile(lq.sl, 99, len(lq.sl))
				gotM := time.Duration(got) * time.Millisecond
				attest.Equal(t, gotM.Minutes(), 3)
			}
			{
				got := percentile(lq.sl, 25, len(lq.sl))
				gotM := time.Duration(got) * time.Millisecond
				attest.Equal(t, gotM, 6*time.Millisecond)
			}
		}
	})
}

func TestLatencyQueue(t *testing.T) {
	t.Parallel()

	t.Run("all samples taken within samplingPeriod", func(t *testing.T) {
		t.Parallel()

		minSampleSize := 10
		lq := newLatencyQueue()
		for i := 1; i <= 1000; i++ {
			lq.add(time.Duration(i) * time.Second)
		}

		got := lq.getPercentile(99, minSampleSize)
		attest.Equal(t, got.Seconds(), 991)
	})

	t.Run("number of samples less than minSampleSize", func(t *testing.T) {
		t.Parallel()

		minSampleSize := 10_000
		lq := newLatencyQueue()
		for i := 1; i <= (minSampleSize / 2); i++ {
			lq.add(time.Duration(i) * time.Second)
		}

		got := lq.getPercentile(99, minSampleSize)
		attest.Zero(t, got)
	})

	t.Run("issues/217: order is preserved", func(t *testing.T) {
		t.Parallel()

		// See: https://github.com/komuw/ong/issues/217

		// 1. Add big latencies.
		lq := newLatencyQueue()
		for i := 1; i <= maxLatencyItems; i++ {
			lq.add(time.Duration(i) * time.Minute)
		}

		// 2. Add very small latency to be latest in the queue.
		smallLatency := 3 * time.Millisecond
		for i := 1; i <= 20; i++ {
			lq.add(smallLatency)
		}

		// 3. Call percentile which may mutate the latency slice.
		_ = percentile(lq.sl, 90, len(lq.sl))

		// 4. resize.
		lq.reSize()

		latest := time.Duration(lq.sl[len(lq.sl)-1]) * time.Millisecond
		secondLatest := time.Duration(lq.sl[len(lq.sl)-1]) * time.Millisecond
		first := time.Duration(lq.sl[0]) * time.Millisecond

		attest.Equal(t, latest, smallLatency)
		attest.Equal(t, secondLatest, smallLatency)
		attest.NotEqual(t, first, smallLatency)
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		lq := newLatencyQueue()

		wg := &sync.WaitGroup{}
		for rN := 0; rN <= 50+config.DefaultLoadShedMinSampleSize; rN++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				lq.add(1 * time.Second)
				lq.reSize()
				lq.getPercentile(99, 3)
			}()
		}
		wg.Wait()
	})
}

func loadShedderBenchmarkHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		latency := time.Duration(rand.IntN(100)+1) * time.Millisecond
		time.Sleep(latency)
		fmt.Fprint(w, "hey")
	}
}

func BenchmarkLoadShedder(b *testing.B) {
	var r int

	wrappedHandler := loadShedder(
		loadShedderBenchmarkHandler(),
		config.DefaultLoadShedSamplingPeriod,
		config.DefaultLoadShedMinSampleSize,
		config.DefaultLoadShedBreachLatency,
		99,
	)

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
