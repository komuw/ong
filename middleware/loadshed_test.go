package middleware

import (
	"fmt"
	"io"
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

// TODO: add a test for an actual case where load testing is triggered.

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

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), msg)
		}
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
			req.Header.Set(loadShedderTestHeader, fmt.Sprint(20))
			wrappedHandler.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			rb, err := io.ReadAll(res.Body)
			attest.Ok(t, err)

			attest.Equal(t, res.StatusCode, http.StatusOK)
			attest.Equal(t, string(rb), msg)
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

func TestPercentile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		{
			lq := latencyQueue{
				sl: []latency{
					{duration: 5 * time.Second},
					{duration: 6 * time.Second},
					{duration: 7 * time.Second},
					{duration: 8 * time.Second},
					{duration: 9 * time.Second},
					{duration: 0 * time.Second},
					{duration: 1 * time.Second},
					{duration: 2 * time.Second},
					{duration: 3 * time.Second},
					{duration: 4 * time.Second},
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
					newLatency(time.Duration(i)*time.Second, time.Now().UTC()),
				)
			}
			got := percentile(lq.sl, 99)
			attest.Equal(t, got.Seconds(), 990.01)
		}
	})
}

func TestLatencyQueue(t *testing.T) {
	t.Run("all samples taken outside samplingPeriod", func(t *testing.T) {
		now := time.Now().UTC()
		samplingPeriod := 10 * time.Millisecond
		minSampleSize := 10

		lq := latencyQueue{}
		for i := 1; i <= 1000; i++ {
			lq.sl = append(
				lq.sl,
				newLatency(time.Duration(i)*time.Second, now),
			)
		}

		got := lq.getP99(now, samplingPeriod, minSampleSize)
		attest.Zero(t, got)
	})

	t.Run("all samples taken within samplingPeriod", func(t *testing.T) {
		now := time.Now().UTC()
		samplingPeriod := 10000 * time.Millisecond
		minSampleSize := 10

		lq := latencyQueue{}
		for i := 1; i <= 1000; i++ {
			lq.sl = append(
				lq.sl,
				newLatency(
					time.Duration(i)*time.Second,
					// negative so that it is in the past.
					// divide by two so that it is within the samplingPeriod
					now.Add(-(samplingPeriod/2)),
				),
			)
		}

		got := lq.getP99(now, samplingPeriod, minSampleSize)
		attest.Equal(t, got.Seconds(), 990.01)
	})

	t.Run("number of samples less than minSampleSize", func(t *testing.T) {
		now := time.Now().UTC()
		samplingPeriod := 10000 * time.Millisecond
		minSampleSize := 10_000

		lq := latencyQueue{}
		for i := 1; i <= (minSampleSize / 2); i++ {
			lq.sl = append(
				lq.sl,
				newLatency(
					time.Duration(i)*time.Second,
					// negative so that it is in the past.
					// divide by two so that it is within the samplingPeriod
					now.Add(-(samplingPeriod/2)),
				),
			)
		}

		got := lq.getP99(now, samplingPeriod, minSampleSize)
		attest.Zero(t, got)
	})

	t.Run("all samples taken in the future", func(t *testing.T) {
		now := time.Now().UTC()
		samplingPeriod := 10000 * time.Millisecond
		minSampleSize := 10

		lq := latencyQueue{}
		for i := 1; i <= 1000; i++ {
			lq.sl = append(
				lq.sl,
				newLatency(
					time.Duration(i)*time.Second,
					// positive so that it is in the future.
					// divide by two so that it is within the samplingPeriod
					now.Add(+(samplingPeriod/2)),
				),
			)
		}

		got := lq.getP99(now, samplingPeriod, minSampleSize)
		attest.Zero(t, got)
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		lq := NewLatencyQueue()

		wg := &sync.WaitGroup{}
		for rN := 0; rN <= 20; rN++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				lq.add(1*time.Second, time.Now().UTC())
				lq.reSize()
				// we can't call lq.size() here since it is not synced.
				// but it is only called by lq.reSize() so it is already tested.
				lq.getP99(time.Now().UTC(), 1*time.Second, 3)
			}()
		}
		wg.Wait()
	})
}
