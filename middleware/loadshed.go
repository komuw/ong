package middleware

import (
	"fmt"
	mathRand "math/rand"
	"net/http"
	"sync"
	"time"

	"golang.org/x/exp/slices"
)

// Most of the code here is insipired by:
//   (a) https://aws.amazon.com/builders-library/using-load-shedding-to-avoid-overload/
//   (b) https://github.com/komuw/celery_experiments/blob/77e6090f7adee0cf800ea5575f2cb22bc798753d/limiter/

const (
	retryAfterHeader = "Retry-After"

	// samplingPeriod is the duration over which we will calculate the latency.
	samplingPeriod = 12 * time.Minute

	// minSampleSize is the minimum number of past requests that have to be available, in the last `samplingPeriod` seconds for us to make a decision.
	// If there were fewer requests(than `minSampleSize`) in the `samplingPeriod`, then we do decide to let things continue without load shedding.
	minSampleSize = 50

	// breachLatency is the p99 latency at which point we start dropping requests.
	// The wikipedia monitoring dashboards are public: https://grafana.wikimedia.org/?orgId=1
	// In there we can see that the p95 response times for http GET requests is ~700ms: https://grafana.wikimedia.org/d/RIA1lzDZk/application-servers-red?orgId=1
	// and the p95 response times for http POST requests is ~900ms.
	// Thus, we'll use a `breachLatency` of ~700ms. We hope we can do better than wikipedia(chuckle emoji.)
	breachLatency = 700 * time.Millisecond

	// retryAfter is how long we expect users to retry requests after getting a http 503, loadShedding.
	retryAfter = samplingPeriod + (5 * time.Minute)

	// resizePeriod is the duration after which we should trim the latencyQueue.
	// It should always be > samplingPeriod
	// we do not want to reduce size of `lq` before a period `> samplingPeriod` otherwise `lq.getP99()` will always return zero.
	resizePeriod = samplingPeriod + (3 * time.Minute)

	// maxLatencyItems is the number of items past which we have to resize the latencyQueue.
	maxLatencyItems = 5_000
)

// loadShedder is a middleware that sheds load based on http response latencies.
func loadShedder(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	// lq should not be a global variable, we want it to be per handler.
	// This is because different handlers(URIs) could have different latencies and we want each to be loadshed independently.
	lq := newLatencyQueue()
	loadShedCheckStart := time.Now().UTC()

	return func(w http.ResponseWriter, r *http.Request) {
		startReq := time.Now().UTC()
		defer func() {
			endReq := time.Now().UTC()
			durReq := endReq.Sub(startReq)
			lq.add(durReq)

			// we do not want to reduce size of `lq` before a period `> samplingPeriod` otherwise `lq.getP99()` will always return zero.
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

		p99 := lq.getP99(minSampleSize)
		if p99 > breachLatency && !sendProbe {
			// drop request
			err := fmt.Errorf("ong/middleware: server is overloaded, retry after %s", retryAfter)
			w.Header().Set(ongMiddlewareErrorHeader, fmt.Sprintf("%s. p99latency: %s. breachLatency: %s", err.Error(), p99, breachLatency))
			w.Header().Set(retryAfterHeader, fmt.Sprintf("%d", int(retryAfter.Seconds()))) // header should be in seconds(decimal-integer).
			http.Error(
				w,
				err.Error(),
				http.StatusServiceUnavailable,
			)
			return
		}

		wrappedHandler(w, r)
	}
}

type latencyQueue struct {
	mu sync.Mutex // protects sl

	/*
		unsafe.Sizeof(sl) == 8bytes.
		latency is how long the operation took(ie latency)

		We do not need to have a field specifying when the latency measurement was taken.
		Since [latencyQueue.reSize] is called oftenly; all the latencies in the queue will
		aways be within `samplingPeriod` give or take.
	*/
	sl []time.Duration
}

func newLatencyQueue() *latencyQueue {
	return &latencyQueue{sl: []time.Duration{}}
}

func (lq *latencyQueue) add(durReq time.Duration) {
	lq.mu.Lock()
	lq.sl = append(lq.sl, durReq)
	lq.mu.Unlock()
}

func (lq *latencyQueue) reSize() {
	lq.mu.Lock()
	defer lq.mu.Unlock()

	size := len(lq.sl)
	if size > maxLatencyItems {
		// Each `latency` struct is 8bytes. So we can afford to have 5_000(40KB)
		half := size / 2
		lq.sl = lq.sl[half:] // retain the latest half.
	}
}

func (lq *latencyQueue) getP99(minSampleSize int) (p99latency time.Duration) {
	lq.mu.Lock()
	defer lq.mu.Unlock()

	lenSl := len(lq.sl)
	if lenSl < minSampleSize {
		// the number of requests in the last `samplingPeriod` seconds is less than
		// is neccessary to make a decision
		return 0 * time.Millisecond
	}

	return percentile(lq.sl, 99, lenSl)
}

func percentile(N []time.Duration, pctl float64, lenSl int) time.Duration {
	newN := slices.Clone(N)

	slices.Sort(newN)
	index := int((pctl / 100) * float64(lenSl))
	return newN[index]
}
