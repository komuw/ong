package middleware

import (
	"fmt"
	"math"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"github.com/komuw/ong/config"
)

// Some of the code here is inspired by(or taken from):
//   (a) https://github.com/FiloSottile/sunlight/blob/661dfebb4a250e8d4a7e53d841fadaa9fc617795/cmd/skylight/main.go#L103-L127 whose license(ISC) can be found here: https://github.com/FiloSottile/sunlight/blob/661dfebb4a250e8d4a7e53d841fadaa9fc617795/LICENSE
//   (b) https://letsencrypt.org/2025/01/30/scaling-rate-limits/
//   (c) https://adam-p.ca/blog/2022/02/ipv6-rate-limiting/

const (
	// sweepPeriod is the maximum duration between two sweeps of expired entries.
	sweepPeriod = 1 * time.Minute
	// maxEntries is the maximum number of entries kept by the ratelimiter.
	// An entry is a [netip.Prefix](32bytes) and a [time.Time](24bytes); with map overhead it is about 100bytes.
	// So 100_000 entries use about 10MB.
	maxEntries = 100_000

	// For IPv6, we also limit the /56 and /48 prefixes that contain the client's /64.
	// They get a higher limit since many legitimate hosts can share them.
	// See: https://adam-p.ca/blog/2022/02/ipv6-rate-limiting/
	ipv6Prefix56Multiplier = 8
	ipv6Prefix48Multiplier = 64
)

// rateLimiter is a middleware that limits requests by IP address.
// It uses the Generic Cell Rate Algorithm (GCRA).
func rateLimiter(
	wrappedHandler http.Handler,
	rateLimit float64,
) http.HandlerFunc {
	rl := newRl(rateLimit, maxEntries)

	return func(w http.ResponseWriter, r *http.Request) {
		allow, retryAfter := rl.allow(ClientIP(r), time.Now())
		if !allow {
			// The header should be in seconds(decimal-integer).
			secs := max(1, int(math.Ceil(retryAfter.Seconds())))
			err := fmt.Errorf("ong/middleware/ratelimiter: rate limited, retry after %ds", secs)
			w.Header().Set(ongMiddlewareErrorHeader, err.Error())
			w.Header().Set(retryAfterHeader, fmt.Sprintf("%d", secs))
			http.Error(
				w,
				err.Error(),
				http.StatusTooManyRequests,
			)
			return
		}

		// Note: We also limit max body size using `http.MaxBytesHandler`
		// See: ong/server

		// todo: also maybe add another limiter for IPv4 subnet.
		//      see limitation: https://github.com/komuw/ong/issues/17#issuecomment-1114551281

		wrappedHandler.ServeHTTP(w, r)
	}
}

// tier is one GCRA limit that applies to a prefix of the client IP address.
type tier struct {
	bits     int           // prefix length.
	interval time.Duration // emission interval; 1/rate.
	// tolerance is the maximum duration that a theoretical arrival time can be ahead of now, for a request to be allowed.
	// It is (burst-1)*interval, so that burst requests are allowed at the same instant.
	tolerance time.Duration
}

// rl is a GCRA ratelimiter keyed by IP address prefix.
// See https://letsencrypt.org/2025/01/30/scaling-rate-limits/
type rl struct {
	mu sync.Mutex // protects tats and lastSweep.
	// tats maps a prefix to its theoretical arrival time(TAT).
	// A missing entry is the same as an entry whose TAT is in the past.
	// +checklocks:mu
	tats map[netip.Prefix]time.Time
	// +checklocks:mu
	lastSweep time.Time

	// The fields below are not changed after construction.
	maxEntries int
	ipv4Tiers  []tier
	ipv6Tiers  []tier
	otherTiers []tier // for empty or invalid IP addresses.
}

func newRl(rateLimit float64, maxEntries int) *rl {
	if rateLimit < 1.0 {
		rateLimit = config.DefaultRateLimit
	}

	// Make sure that the interval of the tier with the highest multiplier is at least one nanosecond.
	rateLimit = min(rateLimit, float64(time.Second)/ipv6Prefix48Multiplier)
	burst := math.Ceil(rateLimit)

	newTier := func(bits int, multiplier float64) tier {
		interval := time.Duration(float64(time.Second) / (rateLimit * multiplier))
		return tier{
			bits:      bits,
			interval:  interval,
			tolerance: interval * time.Duration(burst*multiplier-1),
		}
	}

	return &rl{
		tats:       map[netip.Prefix]time.Time{},
		maxEntries: maxEntries,
		ipv4Tiers:  []tier{newTier(32, 1)},
		ipv6Tiers: []tier{
			newTier(64, 1),
			newTier(56, ipv6Prefix56Multiplier),
			newTier(48, ipv6Prefix48Multiplier),
		},
		otherTiers: []tier{newTier(0, 1)},
	}
}

// allow reports whether a request from ip, at time now, is allowed.
// If it is not allowed, retryAfter is the duration after which the request can be retried.
// A request is allowed only if all the tiers that apply to ip allow it.
// If it is not allowed, no state is changed.
func (r *rl) allow(ip string, now time.Time) (yes bool, retryAfter time.Duration) {
	tiers := r.otherTiers
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		// All empty or invalid addresses share the zero prefix.
		addr = netip.Addr{}
	} else {
		addr = addr.Unmap()
		if addr.Is4() {
			tiers = r.ipv4Tiers
		} else {
			tiers = r.ipv6Tiers
		}
	}

	var keys [3]netip.Prefix
	var newTats [3]time.Time

	r.mu.Lock()
	defer r.mu.Unlock()

	r.sweep(now)

	yes = true
	for i, t := range tiers {
		// Prefix drops any zone. For the zero Addr, it returns the zero Prefix.
		key, _ := addr.Prefix(t.bits)

		tat := r.tats[key]
		if tat.Before(now) {
			tat = now
		}
		limit := now.Add(t.tolerance)
		if tat.After(limit) {
			yes = false
			retryAfter = max(retryAfter, tat.Sub(limit))
			continue
		}
		keys[i] = key
		newTats[i] = tat.Add(t.interval)
	}

	if !yes {
		return false, retryAfter
	}

	for i := range tiers {
		r.tats[keys[i]] = newTats[i]
	}

	return true, 0
}

// sweep removes entries whose TAT is not in the future. This is lossless, since such entries are the same as missing entries.
// If there are still more than maxEntries entries, it removes random entries until 90% of maxEntries remain.
// A client whose entry is removed gets at most one extra burst.
// +checklocks:r.mu
func (r *rl) sweep(now time.Time) {
	if now.Sub(r.lastSweep) < sweepPeriod && len(r.tats) <= r.maxEntries {
		return
	}
	r.lastSweep = now

	for k, tat := range r.tats {
		if !tat.After(now) {
			delete(r.tats, k)
		}
	}

	if len(r.tats) <= r.maxEntries {
		return
	}

	target := r.maxEntries * 9 / 10
	for k := range r.tats {
		if len(r.tats) <= target {
			break
		}
		delete(r.tats, k)
	}
}
