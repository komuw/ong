package middleware

import (
	"fmt"
	mathRand "math/rand"
	"net/http"
	"slices"
	"sync"
	"time"
)

// Most of the code here is inspired by:
//   (a) https://aws.amazon.com/builders-library/using-load-shedding-to-avoid-overload/
//   (b) https://github.com/komuw/celery_experiments/blob/77e6090f7adee0cf800ea5575f2cb22bc798753d/limiter/

const (
	// DefaultLoadShedSamplingPeriod is the duration over which we calculate response latencies by default.
	DefaultLoadShedSamplingPeriod = 12 * time.Minute
	// DefaultLoadShedMinSampleSize is the minimum number of past requests that have to be available, in the last `loadShedSamplingPeriod` for us to make a decision, by default.
	// If there were fewer requests(than `loadShedMinSampleSize`) in the `loadShedSamplingPeriod`, then we do decide to let things continue without load shedding.
	DefaultLoadShedMinSampleSize = 50
	// DefaultLoadShedBreachLatency is the p99 latency at which point we start dropping requests, by default.
	//
	// The value chosen here is because;
	// The wikipedia [monitoring] dashboards are public.
	// In there we can see that the p95 [response] times for http GET requests is ~700ms, & the p95 response times for http POST requests is ~900ms.
	// Thus, we'll use a `loadShedBreachLatency` of ~700ms. We hope we can do better than wikipedia(chuckle emoji.)
	//
	// [monitoring]: https://grafana.wikimedia.org/?orgId=1
	// [response]: https://grafana.wikimedia.org/d/RIA1lzDZk/application-servers-red?orgId=1
	DefaultLoadShedBreachLatency = 700 * time.Millisecond

	// maxLatencyItems is the number of items past which we have to resize the latencyQueue.
	maxLatencyItems = 1_000

	retryAfterHeader = "Retry-After"
)

// loadShedder is a middleware that sheds load based on http response latencies.
func loadShedder(
	wrappedHandler http.Handler,
	loadShedSamplingPeriod time.Duration,
	loadShedMinSampleSize int,
	loadShedBreachLatency time.Duration,
) http.HandlerFunc {
	// lq should not be a global variable, we want it to be per handler.
	// This is because different handlers(URIs) could have different latencies and we want each to be loadshed independently.
	lq := newLatencyQueue()
	loadShedCheckStart := time.Now().UTC()

	if loadShedSamplingPeriod < 1*time.Second {
		loadShedSamplingPeriod = DefaultLoadShedSamplingPeriod
	}
	if loadShedMinSampleSize < 1 {
		loadShedMinSampleSize = DefaultLoadShedMinSampleSize
	}
	if loadShedBreachLatency < 1*time.Nanosecond {
		loadShedBreachLatency = DefaultLoadShedBreachLatency
	}
	var (
		// retryAfter is how long we expect users to retry requests after getting a http 503, loadShedding.
		retryAfter = loadShedSamplingPeriod + (5 * time.Minute)
		// resizePeriod is the duration after which we should trim the latencyQueue.
		// It should always be > loadShedSamplingPeriod
		// we do not want to reduce size of `lq` before a period `> loadShedSamplingPeriod` otherwise `lq.getP99()` will always return zero.
		resizePeriod = loadShedSamplingPeriod + (3 * time.Minute)
	)

	return func(w http.ResponseWriter, r *http.Request) {
		startReq := time.Now().UTC()
		defer func() {
			endReq := time.Now().UTC()
			durReq := endReq.Sub(startReq)
			lq.add(durReq)

			// we do not want to reduce size of `lq` before a period `> loadShedSamplingPeriod` otherwise `lq.getP99()` will always return zero.
			if endReq.Sub(loadShedCheckStart) > resizePeriod {
				// lets reduce the size of latencyQueue
				lq.reSize()
				loadShedCheckStart = endReq
			}
		}()

		sendProbe := false
		{
			// Even if the server is overloaded, we want to send a percentage of the requests through.
			// These requests act as a probe. If the server eventually recovers,
			// these requests will re-populate latencyQueue(`lq`) with lower latencies and thus end the load-shed.
			sendProbe = mathRand.Intn(100) == 1 // let 1% of requests through. NB: Intn(100) is `0-99` ie, 100 is not included.
		}

		p99 := lq.getP99(loadShedMinSampleSize)
		if p99.Milliseconds() > loadShedBreachLatency.Milliseconds() && !sendProbe {
			// drop request
			err := fmt.Errorf("ong/middleware: server is overloaded, retry after %s", retryAfter)
			w.Header().Set(ongMiddlewareErrorHeader, fmt.Sprintf("%s. p99latency: %s. loadShedBreachLatency: %s", err.Error(), p99, loadShedBreachLatency))
			w.Header().Set(retryAfterHeader, fmt.Sprintf("%d", int(retryAfter.Seconds()))) // header should be in seconds(decimal-integer).
			http.Error(
				w,
				err.Error(),
				http.StatusServiceUnavailable,
			)
			return
		}

		wrappedHandler.ServeHTTP(w, r)
	}
}

type latencyQueue struct {
	mu sync.Mutex // protects sl
	/*
		unsafe.Sizeof(sl) == 8bytes.
		latency is how long the operation took(ie latency)

		We do not need to have a field specifying when the latency measurement was taken.
		Since [latencyQueue.reSize] is called oftenly; all the latencies in the queue will
		aways be within `loadShedSamplingPeriod` give or take.
	*/
	// +checklocks:mu
	sl []int64
}

func newLatencyQueue() *latencyQueue {
	return &latencyQueue{sl: []int64{}}
}

func (lq *latencyQueue) add(durReq time.Duration) {
	lq.mu.Lock()
	lq.sl = append(lq.sl, durReq.Milliseconds())
	lq.mu.Unlock()
}

func (lq *latencyQueue) reSize() {
	lq.mu.Lock()
	defer lq.mu.Unlock()

	size := len(lq.sl)
	if size > maxLatencyItems {
		// Each `latency` struct is 8bytes. So we can afford to have upto 1_000(8KB) items.
		half := size / 2
		lq.sl = lq.sl[half:] // retain the latest half.
	}
}

func (lq *latencyQueue) getP99(loadShedMinSampleSize int) (p99latency time.Duration) {
	lq.mu.Lock()
	defer lq.mu.Unlock()

	lenSl := len(lq.sl)
	if lenSl < loadShedMinSampleSize {
		// the number of requests in the last `loadShedSamplingPeriod` seconds is less than
		// is neccessary to make a decision
		return 0 * time.Millisecond
	}

	return time.Duration(percentile(lq.sl, 99, lenSl)) * time.Millisecond
}

func percentile(N []int64, pctl float64, lenSl int) int64 {
	newN := slices.Clone(N)

	slices.Sort(newN)
	index := int((pctl / 100) * float64(lenSl))
	return newN[index]
}
