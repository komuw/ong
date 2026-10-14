package middleware

import (
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
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
			wg.Go(func() {
				runhandler()
			})
		}
		wg.Wait()
	})

	t.Run("concurrency safe after percentileInterval", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			msg := "hello"
			samplingPeriod := 1 * time.Second
			wrappedHandler := loadShedder(
				someLoadShedderHandler(msg),
				samplingPeriod,
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
				wg.Go(func() {
					runhandler()
				})
			}
			wg.Wait()

			// Move past percentileInterval so that the handlers calculate the percentile again.
			time.Sleep(percentileInterval + time.Second)

			for rN := 0; rN <= 50+config.DefaultLoadShedMinSampleSize; rN++ {
				wg.Go(func() {
					runhandler()
				})
			}
			wg.Wait()
		})
	})

	t.Run("sheds load", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			msg := "hello"
			minSampleSize := 5
			wrappedHandler := loadShedder(
				someLoadShedderHandler(msg),
				config.DefaultLoadShedSamplingPeriod,
				minSampleSize,
				10*time.Millisecond, // breach latency
				50,
			)

			run := func(latencyMs string) int {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
				req.Header.Set(loadShedderTestHeader, latencyMs)
				wrappedHandler.ServeHTTP(rec, req)

				res := rec.Result()
				defer res.Body.Close()
				return res.StatusCode
			}

			// Slow requests. They are not shed, since there are not enough samples yet.
			for range minSampleSize {
				attest.Equal(t, run("50"), http.StatusOK)
			}

			// Move past percentileInterval so that the percentile is calculated again.
			time.Sleep(percentileInterval + time.Second)

			// 1% of requests are let through as probes. So we only expect most requests to be shed.
			shed := 0
			for range 10 {
				if run("1") == http.StatusServiceUnavailable {
					shed = shed + 1
				}
			}
			attest.True(t, shed >= 5, attest.Sprint(shed))
		})
	})

	t.Run("shed requests do not stop load shedding", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			msg := "hello"
			minSampleSize := 5
			wrappedHandler := loadShedder(
				someLoadShedderHandler(msg),
				config.DefaultLoadShedSamplingPeriod,
				minSampleSize,
				10*time.Millisecond, // breach latency
				50,
			)

			run := func(latencyMs string) int {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
				req.Header.Set(loadShedderTestHeader, latencyMs)
				wrappedHandler.ServeHTTP(rec, req)

				res := rec.Result()
				defer res.Body.Close()
				return res.StatusCode
			}

			// The server is slow.
			for range minSampleSize {
				attest.Equal(t, run("50"), http.StatusOK)
			}
			time.Sleep(percentileInterval + time.Second)

			// Many requests are shed. If each one added a sample of about 0ms, the percentile would be lower than the breach latency.
			for range 20 {
				_ = run("1")
			}
			time.Sleep(percentileInterval + time.Second)

			// The load shedding has not stopped.
			shed := 0
			for range 10 {
				if run("1") == http.StatusServiceUnavailable {
					shed = shed + 1
				}
			}
			attest.True(t, shed >= 5, attest.Sprint(shed))
		})
	})

	t.Run("load shedding stops after samplingPeriod", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			msg := "hello"
			minSampleSize := 5
			samplingPeriod := 1 * time.Minute
			wrappedHandler := loadShedder(
				someLoadShedderHandler(msg),
				samplingPeriod,
				minSampleSize,
				10*time.Millisecond, // breach latency
				50,
			)

			run := func(latencyMs string) int {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/someUri", nil)
				req.Header.Set(loadShedderTestHeader, latencyMs)
				wrappedHandler.ServeHTTP(rec, req)

				res := rec.Result()
				defer res.Body.Close()
				return res.StatusCode
			}

			// The server is slow.
			for range minSampleSize {
				attest.Equal(t, run("50"), http.StatusOK)
			}
			time.Sleep(percentileInterval + time.Second)

			shed := 0
			for range 10 {
				if run("1") == http.StatusServiceUnavailable {
					shed = shed + 1
				}
			}
			attest.True(t, shed >= 5, attest.Sprint(shed))

			// The slow samples are now older than samplingPeriod, so they are not used.
			time.Sleep(samplingPeriod + percentileInterval)
			for range 10 {
				attest.Equal(t, run("1"), http.StatusOK)
			}
		})
	})
}

// samples returns the latencies of the samples in lq, from the oldest to the newest.
func samples(lq *latencyQueue) []int64 {
	lq.mu.Lock()
	defer lq.mu.Unlock()

	ordered := lq.buf[:lq.next]
	if lq.full {
		ordered = append(slices.Clone(lq.buf[lq.next:]), lq.buf[:lq.next]...)
	}

	latencies := []int64{}
	for _, s := range ordered {
		latencies = append(latencies, s.latencyMs)
	}
	return latencies
}

func TestPercentile(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		getPercentile := func(pctl float64, durs []time.Duration) time.Duration {
			lq := newLatencyQueue(pctl, 1, time.Hour)
			for _, dur := range durs {
				lq.add(time.Now(), dur)
			}
			return lq.getPercentile(time.Now())
		}

		{
			got := getPercentile(25, []time.Duration{
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
			})
			attest.Equal(t, got, 2*time.Second)
		}
		{
			durs := []time.Duration{}
			for i := 1; i <= 1000; i++ {
				durs = append(durs, time.Duration(i)*time.Second)
			}
			attest.Equal(t, getPercentile(99, durs).Seconds(), 991)

			slices.Reverse(durs)
			attest.Equal(t, getPercentile(99, durs).Seconds(), 991)
		}

		{ // different duration units mixed in.
			durs := []time.Duration{
				3 * time.Minute,
				1 * time.Second,
				4 * time.Microsecond,
				6 * time.Millisecond,
			}
			attest.Equal(t, getPercentile(99, durs).Minutes(), 3)
			attest.Equal(t, getPercentile(25, durs), 6*time.Millisecond)
		}

		{
			N := []int64{315, 17, 254, 1215, 306, 7, 318, 13, 316, 143, 209, 315, 46, 42, 43, 544, 46, 300, 325, 905, 325, 718, 908, 527, 65, 637, 48, 31, 43, 318, 1030, 326, 9, 318, 10, 343, 1042, 11, 267, 706, 43, 397, 322, 971, 317, 7, 40, 390, 318, 9, 331, 410, 749, 46, 489, 334, 585, 47}
			slices.Sort(N)
			got := percentile(N, 99)
			attest.Equal(t, got, 1215)
		}
	})

	t.Run("range of percentiles", func(t *testing.T) {
		t.Parallel()

		N := []int64{1, 2, 3, 4}
		tests := []struct {
			pctl float64
			want int64
		}{
			{pctl: 0, want: 1},
			{pctl: 50, want: 3},
			{pctl: 99.9, want: 4},
			{pctl: 100, want: 4}, // index is clamped to the last item.
		}
		for _, tt := range tests {
			got := percentile(N, tt.pctl)
			attest.Equal(t, got, tt.want, attest.Sprint(tt.pctl))
		}
	})
}

func TestLatencyQueue(t *testing.T) {
	t.Parallel()

	t.Run("number of samples less than minSampleSize", func(t *testing.T) {
		t.Parallel()

		minSampleSize := 10_000 // more than maxLatencyItems.
		lq := newLatencyQueue(99, minSampleSize, time.Hour)
		now := time.Now()
		for i := 1; i <= (minSampleSize / 2); i++ {
			lq.add(time.Now(), time.Duration(i)*time.Second)
		}
		attest.Zero(t, lq.getPercentile(now))

		// The queue is big enough to keep minSampleSize samples.
		for i := (minSampleSize / 2) + 1; i <= minSampleSize; i++ {
			lq.add(time.Now(), time.Duration(i)*time.Second)
		}
		attest.Equal(t, len(samples(lq)), minSampleSize)
		attest.Equal(t, lq.getPercentile(now.Add(percentileInterval)).Seconds(), 9901)
	})

	t.Run("size is limited", func(t *testing.T) {
		t.Parallel()

		lq := newLatencyQueue(99, 1, time.Hour)
		for i := 1; i <= 3*maxLatencyItems; i++ {
			lq.add(time.Now(), time.Duration(i)*time.Millisecond)
		}

		got := samples(lq)
		attest.Equal(t, len(got), maxLatencyItems)
		attest.Equal(t, cap(lq.buf), maxLatencyItems)
		// Only the latest samples are kept.
		attest.Equal(t, got[0], int64(2*maxLatencyItems+1))
		attest.Equal(t, got[len(got)-1], int64(3*maxLatencyItems))
	})

	t.Run("new samples replace old samples", func(t *testing.T) {
		t.Parallel()

		lq := newLatencyQueue(99, 1, time.Hour)
		for range maxLatencyItems {
			lq.add(time.Now(), 1*time.Minute)
		}
		now := time.Now()
		attest.Equal(t, lq.getPercentile(now), 1*time.Minute)

		for range maxLatencyItems {
			lq.add(time.Now(), 3*time.Millisecond)
		}
		attest.Equal(t, lq.getPercentile(now.Add(percentileInterval)), 3*time.Millisecond)
	})

	t.Run("issues/217: order is preserved", func(t *testing.T) {
		t.Parallel()

		// See: https://github.com/komuw/ong/issues/217

		// 1. Add big latencies.
		lq := newLatencyQueue(90, 1, time.Hour)
		for i := 1; i <= maxLatencyItems; i++ {
			lq.add(time.Now(), time.Duration(i)*time.Minute)
		}

		// 2. Add very small latency to be latest in the queue.
		smallLatency := 3 * time.Millisecond
		for i := 1; i <= 20; i++ {
			lq.add(time.Now(), smallLatency)
		}

		// 3. Calculate the percentile, which sorts the samples.
		_ = lq.getPercentile(time.Now())

		got := samples(lq)
		latest := time.Duration(got[len(got)-1]) * time.Millisecond
		secondLatest := time.Duration(got[len(got)-2]) * time.Millisecond
		first := time.Duration(got[0]) * time.Millisecond

		attest.Equal(t, latest, smallLatency)
		attest.Equal(t, secondLatest, smallLatency)
		attest.NotEqual(t, first, smallLatency)
	})

	t.Run("result is re-used within percentileInterval", func(t *testing.T) {
		t.Parallel()

		lq := newLatencyQueue(99, 1, time.Hour)
		for range 100 {
			lq.add(time.Now(), 1*time.Second)
		}
		start := time.Now()
		attest.Equal(t, lq.getPercentile(start), 1*time.Second)

		for range maxLatencyItems {
			lq.add(time.Now(), 1*time.Minute)
		}
		// Not calculated again.
		attest.Equal(t, lq.getPercentile(start.Add(percentileInterval-time.Millisecond)), 1*time.Second)
		// Calculated again.
		attest.Equal(t, lq.getPercentile(start.Add(percentileInterval)), 1*time.Minute)
	})

	t.Run("old samples are not used", func(t *testing.T) {
		t.Parallel()

		samplingPeriod := 1 * time.Minute
		start := time.Now()

		{
			lq := newLatencyQueue(99, 1, samplingPeriod)
			for range 10 {
				lq.add(start, 1*time.Minute)
			}
			later := start.Add(2 * samplingPeriod)
			for range 10 {
				lq.add(later, 1*time.Millisecond)
			}
			attest.Equal(t, lq.getPercentile(later), 1*time.Millisecond)
		}

		{
			lq := newLatencyQueue(99, 1, samplingPeriod)
			for range 10 {
				lq.add(start, 1*time.Minute)
			}
			attest.Equal(t, lq.getPercentile(start.Add(samplingPeriod)), 1*time.Minute) // still inside samplingPeriod.

			later := start.Add(samplingPeriod + percentileInterval + time.Millisecond)
			attest.Zero(t, lq.getPercentile(later)) // no samples in the last samplingPeriod.
		}
	})

	t.Run("minSampleSize counts only samples in samplingPeriod", func(t *testing.T) {
		t.Parallel()

		samplingPeriod := 1 * time.Minute
		start := time.Now()
		lq := newLatencyQueue(99, 5, samplingPeriod)
		for range 10 {
			lq.add(start, 1*time.Minute)
		}
		later := start.Add(2 * samplingPeriod)
		for range 3 {
			lq.add(later, 1*time.Millisecond)
		}
		attest.Zero(t, lq.getPercentile(later))
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		lq := newLatencyQueue(99, 3, time.Hour)

		wg := &sync.WaitGroup{}
		for rN := 0; rN <= 50+config.DefaultLoadShedMinSampleSize; rN++ {
			wg.Go(func() {
				lq.add(time.Now(), 1*time.Second)
				lq.getPercentile(time.Now())
			})
		}
		wg.Wait()
	})
}

// TestLatencyQueueAllocs is not a parallel test, since testing.AllocsPerRun cannot be used in parallel tests.
func TestLatencyQueueAllocs(t *testing.T) {
	// A long samplingPeriod, so that all the samples are used even though now moves forward on each run.
	lq := newLatencyQueue(99, 1, 100*24*time.Hour)
	now := time.Now()
	for range maxLatencyItems {
		lq.add(now, 1*time.Second)
	}

	allocs := testing.AllocsPerRun(100, func() {
		now = now.Add(percentileInterval) // calculate the percentile on each run.
		lq.add(now, 2*time.Second)
		_ = lq.getPercentile(now)
	})
	attest.Zero(t, allocs)
}

func BenchmarkLatencyQueue(b *testing.B) {
	// A long samplingPeriod, so that all the samples are used even though now moves forward on each run.
	lq := newLatencyQueue(99, 1, 100*365*24*time.Hour)
	now := time.Now()
	for i := range maxLatencyItems {
		lq.add(now, time.Duration(i)*time.Millisecond)
	}

	b.Run("getPercentile cached", func(b *testing.B) {
		_ = lq.getPercentile(now)
		b.ReportAllocs()
		for range b.N {
			lq.add(now, 5*time.Millisecond)
			_ = lq.getPercentile(now)
		}
	})

	b.Run("getPercentile calculated", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			now = now.Add(percentileInterval)
			lq.add(now, 5*time.Millisecond)
			_ = lq.getPercentile(now)
		}
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
	for range 100 {
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
