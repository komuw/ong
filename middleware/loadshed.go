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

// TODO: checkout https://aws.amazon.com/builders-library/using-load-shedding-to-avoid-overload/

const (
	retryAfterHeader = "Retry-After"
)

// LoadShedder is a middleware that sheds load based on response latencies.
func LoadShedder(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	mathRandFromTime := mathRand.New(mathRand.NewSource(time.Now().UTC().UnixNano()))
	lq := NewLatencyQueue()

	// TODO: make the following variables configurable(or have good deafult values.); minSampleSize, samplingPeriod, breachLatency
	//
	// The minimum number of past requests that have to be available, in the last `samplingPeriod` seconds for us to make a decision.
	// If there were fewer requests in the `samplingPeriod`, then we do decide to let things continue without load shedding.
	minSampleSize := 10
	samplingPeriod := 2 * time.Second
	// The p99 latency(in milliSeconds) at which point we start dropping requests.
	breachLatency := 66 * time.Millisecond

	loadShedCheckStart := time.Now().UTC()
	return func(w http.ResponseWriter, r *http.Request) {
		startReq := time.Now().UTC()
		defer func() {
			endReq := time.Now().UTC()
			durReq := endReq.Sub(startReq)
			lq.add(durReq, endReq)

			// we do not want to reduce size of `lq` before atleast `samplingPeriod` otherwise `lq.getP99()` will always return zero.
			if endReq.Sub(loadShedCheckStart) > (2 * samplingPeriod) {
				// lets reduce the size of latencyQueue
				lq.reSize()
				loadShedCheckStart = endReq
			}
		}()

		// Even if the server is overloaded, we want to send 1% of the requests through.
		// These requests act as a probe. If the server eventually recovers,
		// these requests will re-populate latencyQueue(`lq`) with lower latencies and thus end the load-shed.
		sendProbe := mathRandFromTime.Intn(100) == 1

		p99 := lq.getP99(startReq, samplingPeriod, minSampleSize)
		fmt.Println("p99: ", p99)
		if p99 > breachLatency && !sendProbe {
			// drop request
			retryAfter := 15 * time.Minute
			err := fmt.Errorf("server is overloaded, retry after %s", retryAfter)
			w.Header().Set(gowebMiddlewareErrorHeader, fmt.Sprintf("%s. p99latency: %s", err.Error(), p99))
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

Note that if `at`` was a `time.Time` `unsafe.Sizeof` would report 32bytes.
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

func NewLatencyQueue() *latencyQueue {
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

	size := lq.size()
	if size > 5_000 {
		// Each `latency` struct is 16bytes. So we can afford to have 5_000 of them at 80KB
		half := size / 2
		lq.sl = lq.sl[half:] // retain the latest half.
	}
}

func (lq *latencyQueue) size() int {
	// we dont need to lock here since all calls of this are in `reSize()`
	return len(lq.sl)
}

func (lq *latencyQueue) getP99(now time.Time, samplingPeriod time.Duration, minSampleSize int) (p99latency time.Duration) {
	lq.mu.Lock()
	defer lq.mu.Unlock()

	_hold := []latency{}
	for _, lat := range lq.sl {
		at := time.Unix(lat.at, 0).UTC()
		elapsed := now.Sub(at)
		// fmt.Println(at, elapsed, samplingPeriod)
		if elapsed < 0 {
			// `at` is in the future. Ignore those values
			break
		}
		if elapsed <= samplingPeriod {
			// is the elapsed time within the samplingPeriod?
			_hold = append(_hold, lat)
		}
	}

	// fmt.Println(len(_hold))
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

	if f == c {
		return N[int(k)].duration
	}

	d0 := float64(N[int(f)].duration.Nanoseconds()) * (c - k)
	d1 := float64(N[int(c)].duration.Nanoseconds()) * (k - f)
	d2 := d0 + d1

	return time.Duration(d2) * time.Nanosecond
}
