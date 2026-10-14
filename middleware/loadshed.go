package middleware

import (
	"fmt"
	mathRand "math/rand/v2"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/komuw/ong/config"
)

// Some of the code here is inspired by:
//   (a) https://aws.amazon.com/builders-library/using-load-shedding-to-avoid-overload/
//   (b) https://github.com/komuw/celery_experiments/blob/77e6090f7adee0cf800ea5575f2cb22bc798753d/limiter/

const (
	// maxLatencyItems is the maximum number of latency samples that are kept for each loadShedder.
	// When the queue is full, each new sample replaces the oldest one.
	// Each sample is 16 bytes, and each item of the slice used for sorting is 8 bytes. So they use 24*maxLatencyItems bytes(96KB).
	maxLatencyItems = 4096
	// percentileInterval is how often the percentile is calculated again.
	// Between calculations, the last result is used. This is so that each request does not sort all the samples.
	percentileInterval = 1 * time.Second

	retryAfterHeader = "Retry-After"

	pprofEndpoints = "/debug/pprof"
)

// loadShedder is a middleware that sheds load based on http response latencies.
// It does not take into account latencies for requests to the pprof endpoints.
func loadShedder(
	wrappedHandler http.Handler,
	loadShedSamplingPeriod time.Duration,
	loadShedMinSampleSize int,
	loadShedBreachLatency time.Duration,
	loadShedPercentile float64,
) http.HandlerFunc {
	if loadShedSamplingPeriod < 1*time.Second {
		loadShedSamplingPeriod = config.DefaultLoadShedSamplingPeriod
	}
	if loadShedMinSampleSize < 1 {
		loadShedMinSampleSize = config.DefaultLoadShedMinSampleSize
	}
	if loadShedBreachLatency < 1*time.Nanosecond {
		loadShedBreachLatency = config.DefaultLoadShedBreachLatency
	}
	if loadShedPercentile < 0 { // zero is a valid user choice.
		loadShedPercentile = config.DefaultLoadShedPercentile
	}

	// lq should not be a global variable, we want it to be per handler.
	// This is because different handlers(URIs) could have different latencies and we want each to be loadshed independently.
	lq := newLatencyQueue(loadShedPercentile, loadShedMinSampleSize, loadShedSamplingPeriod)

	// retryAfter is how long we expect users to retry requests after getting a http 503, loadShedding.
	retryAfter := loadShedSamplingPeriod + (5 * time.Minute)

	return func(w http.ResponseWriter, r *http.Request) {
		startReq := time.Now()

		sendProbe := false
		{
			// Even if the server is overloaded, we want to send a percentage of the requests through.
			// These requests act as a probe. If the server eventually recovers,
			// these requests will re-populate latencyQueue(`lq`) with lower latencies and thus end the load-shed.
			sendProbe = mathRand.IntN(100) == 1 // let 1% of requests through. NB: Intn(100) is `0-99` ie, 100 is not included.
		}

		pctl := lq.getPercentile(startReq)
		if pctl.Milliseconds() > loadShedBreachLatency.Milliseconds() && !sendProbe {
			// drop request
			err := fmt.Errorf("ong/middleware/loadshed: server is overloaded, retry after %s", retryAfter)
			w.Header().Set(ongMiddlewareErrorHeader, fmt.Sprintf("%s. %vPercentile: %s. loadShedBreachLatency: %s", err.Error(), loadShedPercentile, pctl, loadShedBreachLatency))
			w.Header().Set(retryAfterHeader, fmt.Sprintf("%d", int(retryAfter.Seconds()))) // header should be in seconds(decimal-integer).
			http.Error(
				w,
				err.Error(),
				http.StatusServiceUnavailable,
			)
			return
		}

		// This is after the request could have been shed. A request that is shed takes about 0ms; if it was
		// added as a sample, it would make the percentile lower and stop the load shedding even if the server is still slow.
		defer func() {
			if !strings.HasPrefix(r.URL.Path, pprofEndpoints) {
				// Ignore latencies of pprof endpoints.
				endReq := time.Now()
				lq.add(endReq, endReq.Sub(startReq))
			}
		}()

		wrappedHandler.ServeHTTP(w, r)
	}
}

// latencyQueue keeps the latencies of the latest requests(see maxLatencyItems), and calculates a percentile of them.
//
// It has a fixed size(a ring buffer), so its memory use and the time to sort it have a limit.
// The percentile is calculated at most one time in each percentileInterval. Each request uses the last result.
// Only the samples that were added in the last samplingPeriod are used.
type latencyQueue struct {
	// pctl is the percentile to calculate. It is in the range 0 to 100.
	pctl float64
	// minSampleSize is the minimum number of samples(in the last samplingPeriod) that are necessary to calculate the percentile.
	minSampleSize int
	// samplingPeriod is the age after which a sample is not used.
	samplingPeriod time.Duration

	mu sync.Mutex // protects all the fields below.
	// buf is a ring buffer of samples. len(buf) == max(maxLatencyItems, minSampleSize).
	// +checklocks:mu
	buf []sample
	// next is the index in buf where the next sample is written.
	// +checklocks:mu
	next int
	// full is true after buf has been filled one time. Then all the items in buf are samples.
	// +checklocks:mu
	full bool
	// scratch is re-used to sort the samples, so that a calculation does not allocate memory.
	// +checklocks:mu
	scratch []int64
	// result is the last calculated percentile.
	// +checklocks:mu
	result time.Duration
	// resultAt is the time at which result was calculated.
	// +checklocks:mu
	resultAt time.Time
}

// sample is the latency of one request.
type sample struct {
	// atMs is the time(unix milliseconds) at which the sample was added.
	atMs int64
	// latencyMs is the latency of the request in milliseconds.
	latencyMs int64
}

func newLatencyQueue(pctl float64, minSampleSize int, samplingPeriod time.Duration) *latencyQueue {
	// The queue should be able to keep at least minSampleSize samples, otherwise the percentile would never be calculated.
	size := max(maxLatencyItems, minSampleSize)
	return &latencyQueue{
		pctl:           pctl,
		minSampleSize:  minSampleSize,
		samplingPeriod: samplingPeriod,
		buf:            make([]sample, size),
		scratch:        make([]int64, 0, size),
	}
}

// add adds a sample that was taken at time now. If the queue is full, the sample replaces the oldest one.
func (lq *latencyQueue) add(now time.Time, durReq time.Duration) {
	lq.mu.Lock()
	defer lq.mu.Unlock()

	lq.buf[lq.next] = sample{atMs: now.UnixMilli(), latencyMs: durReq.Milliseconds()}
	lq.next = (lq.next + 1) % len(lq.buf)
	if lq.next == 0 {
		lq.full = true
	}
}

// getPercentile returns the lq.pctl percentile of the samples that were added in the last lq.samplingPeriod.
// It returns zero if there are fewer than lq.minSampleSize of those samples.
// It calculates the percentile again only if more than percentileInterval has passed since the last calculation.
func (lq *latencyQueue) getPercentile(now time.Time) time.Duration {
	lq.mu.Lock()
	defer lq.mu.Unlock()

	if !lq.resultAt.IsZero() && now.Sub(lq.resultAt) < percentileInterval {
		return lq.result
	}
	lq.resultAt = now

	n := lq.next
	if lq.full {
		n = len(lq.buf)
	}

	// Copy the latencies, so that the order of samples in buf(oldest to newest) does not change when they are sorted.
	oldest := now.Add(-lq.samplingPeriod).UnixMilli()
	lq.scratch = lq.scratch[:0]
	for _, s := range lq.buf[:n] {
		if s.atMs >= oldest {
			lq.scratch = append(lq.scratch, s.latencyMs)
		}
	}

	if len(lq.scratch) < lq.minSampleSize {
		// the number of samples in the last samplingPeriod is less than is necessary to make a decision.
		lq.result = 0
		return lq.result
	}

	slices.Sort(lq.scratch)
	lq.result = time.Duration(percentile(lq.scratch, lq.pctl)) * time.Millisecond

	return lq.result
}

// percentile returns the pctl percentile of sorted. sorted should be sorted in increasing order and should not be empty.
func percentile(sorted []int64, pctl float64) int64 {
	index := int((pctl / 100) * float64(len(sorted)))
	// When pctl is 100, index is equal to len(sorted). That is out of range, so use the last item(the largest latency).
	index = min(index, len(sorted)-1)
	return sorted[index]
}
