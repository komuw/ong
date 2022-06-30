package middleware

import (
	"fmt"
	"math"
	mathRand "math/rand"
	"net/http"
	"sort"
	"sync"
	"time"
)

// Most of the code here is insipired by:
//   (a) https://aws.amazon.com/builders-library/using-load-shedding-to-avoid-overload/
//   (b) https://github.com/komuw/celery_experiments/blob/77e6090f7adee0cf800ea5575f2cb22bc798753d/limiter/

const (
	retryAfterHeader = "Retry-After"
)

// LoadShedder is a middleware that sheds load based on response latencies.
func LoadShedder(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	mathRand.Seed(time.Now().UTC().UnixNano())
	lq := newLatencyQueue()

	/*
		The wikipedia monitoring dashboards are public: https://grafana.wikimedia.org/?orgId=1
		In there we can see that the p95 response times for http GET requests is ~700ms: https://grafana.wikimedia.org/d/RIA1lzDZk/application-servers-red?orgId=1
		and the p95 response times for http POST requests is ~900ms.

		Thus, we'll use a `breachLatency` of ~700ms. We hope we can do better than wikipedia(chuckle emoji.)
	*/

	// samplingPeriod is the duration over which we will calculate the latency.
	samplingPeriod := 12 * time.Minute
	// minSampleSize is the minimum number of past requests that have to be available, in the last `samplingPeriod` seconds for us to make a decision.
	// If there were fewer requests(than `minSampleSize`) in the `samplingPeriod`, then we do decide to let things continue without load shedding.
	minSampleSize := 50
	// breachLatency is the p99 latency at which point we start dropping requests.
	breachLatency := 700 * time.Millisecond

	// retryAfter is how long we expect users to retry requests after getting a http 503, loadShedding.
	retryAfter := samplingPeriod + (3 * time.Minute)

	loadShedCheckStart := time.Now().UTC()

	return func(w http.ResponseWriter, r *http.Request) {
		startReq := time.Now().UTC()
		defer func() {
			endReq := time.Now().UTC()
			durReq := endReq.Sub(startReq)
			lq.add(durReq, endReq)

			// we do not want to reduce size of `lq` before a period `> samplingPeriod` otherwise `lq.getP99()` will always return zero.
			if endReq.Sub(loadShedCheckStart) > (samplingPeriod + (3 * time.Minute)) {
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

		p99 := lq.getP99(startReq, samplingPeriod, minSampleSize)
		if p99 > breachLatency && !sendProbe {
			// drop request
			err := fmt.Errorf("server is overloaded, retry after %s", retryAfter)
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

/*
unsafe.Sizeof(latency{}) == 16bytes.

Note that if `latency.at` was a `time.Time` `unsafe.Sizeof` would report 32bytes.
However, this wouldn't be the true size since `unsafe.Sizeof` does not 'chase' pointers and time.Time has some.
*/
type latency struct {
	// duration is how long the operation took(ie latency)
	duration time.Duration
	// at is the time at which this operation took place.
	// We could have ideally used a `time.Time` as its type; but we wanted the latency struct to be minimal in size.
	at int64
}

func newLatency(d time.Duration, a time.Time) latency {
	return latency{
		duration: d,
		at:       a.Unix(),
	}
}

func (l latency) String() string {
	return fmt.Sprintf("{dur: %s, at: %s}", l.duration, time.Unix(l.at, 0).UTC())
}

type latencyQueue struct {
	mu sync.Mutex // protects sl
	sl []latency
}

func newLatencyQueue() *latencyQueue {
	return &latencyQueue{
		sl: []latency{},
	}
}

func (lq *latencyQueue) add(durReq time.Duration, endReq time.Time) {
	lq.mu.Lock()
	lq.sl = append(lq.sl, newLatency(durReq, endReq))
	lq.mu.Unlock()
}

func (lq *latencyQueue) reSize() {
	lq.mu.Lock()
	defer lq.mu.Unlock()

	size := len(lq.sl)
	if size > 5_000 {
		// Each `latency` struct is 16bytes. So we can afford to have 5_000(80KB)
		half := size / 2
		lq.sl = lq.sl[half:] // retain the latest half.
	}
}

// todo: refactor this and its dependents.
// currently they consume 9.04MB and 80ms as measured by the `BenchmarkAllMiddlewares` benchmark.
func (lq *latencyQueue) getP99(now time.Time, samplingPeriod time.Duration, minSampleSize int) (p99latency time.Duration) {
	lq.mu.Lock()
	defer lq.mu.Unlock()

	_hold := []latency{}
	for _, lat := range lq.sl {
		at := time.Unix(lat.at, 0).UTC()
		elapsed := now.Sub(at)
		if elapsed < 0 {
			// `at` is in the future. Ignore those values
			break
		}
		if elapsed <= samplingPeriod {
			// is the elapsed time within the samplingPeriod?
			_hold = append(_hold, lat)
		}
	}

	if len(_hold) < minSampleSize {
		// the number of requests in the last `samplingPeriod` seconds is less than
		// is neccessary to make a decision
		return 0 * time.Millisecond
	}

	return percentile(_hold, 99)
}

func percentile(N []latency, pctl float64) time.Duration {
	// This is taken from:
	// https://github.com/komuw/celery_experiments/blob/77e6090f7adee0cf800ea5575f2cb22bc798753d/limiter/limit.py#L253-L280
	//
	// todo: use something better like: https://github.com/influxdata/tdigest
	//
	pctl = pctl / 100

	sort.Slice(N, func(i, j int) bool {
		return N[i].duration < N[j].duration
	})

	k := float64((len(N) - 1)) * pctl
	f := math.Floor(k)
	c := math.Ceil(k)

	if int(f) == int(c) { // golangci-lint complained about comparing floats.
		return N[int(k)].duration
	}

	d0 := float64(N[int(f)].duration.Nanoseconds()) * (c - k)
	d1 := float64(N[int(c)].duration.Nanoseconds()) * (k - f)
	d2 := d0 + d1

	return time.Duration(d2) * time.Nanosecond
}
