package middleware

import (
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"slices"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// idleAfter is how long an IP's bucket is kept after its last request.
const idleAfter = 10 * time.Minute

type limiter struct {
	mu        sync.Mutex
	every     rate.Limit
	burst     int
	clients   map[string]*client
	lastSweep time.Time
}

type client struct {
	bucket   *rate.Limiter
	lastSeen time.Time
}

func newLimiter(every rate.Limit, burst int) *limiter {
	return &limiter{every: every, burst: burst, clients: map[string]*client{}, lastSweep: time.Now()}
}

// allow reports whether ip may proceed, and if not, how long until it may.
func (l *limiter) allow(ip string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.lastSweep) > time.Minute {
		for k, c := range l.clients {
			if now.Sub(c.lastSeen) > idleAfter {
				delete(l.clients, k)
			}
		}
		l.lastSweep = now
	}
	c, ok := l.clients[ip]
	if !ok {
		c = &client{bucket: rate.NewLimiter(l.every, l.burst)}
		l.clients[ip] = c
	}
	c.lastSeen = now
	r := c.bucket.ReserveN(now, 1)
	if delay := r.DelayFrom(now); delay > 0 {
		r.CancelAt(now)
		return false, delay
	}
	return true, 0
}

// RateLimit allows each client IP `burst` requests at once, refilling at
// `every`. Over the limit it answers 429 with Retry-After.
//
// ponytail: in-memory buckets keyed by RemoteAddr. Behind a reverse proxy every
// request shares the proxy's IP, so read X-Real-IP from a trusted proxy then;
// with several API instances, move the buckets to a shared store.
func RateLimit(every rate.Limit, burst int) func(http.Handler) http.Handler {
	l := newLimiter(every, burst)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ok, wait := l.allow(clientIP(r), time.Now()); !ok {
				w.Header().Set("Retry-After", fmt.Sprint(int(math.Ceil(wait.Seconds()))))
				writeError(w, http.StatusTooManyRequests, "too many requests")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CORS lets the listed browser origins call the API with a Bearer token.
func CORS(origins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Vary", "Origin")
			origin := r.Header.Get("Origin")
			if origin == "" || !slices.Contains(origins, origin) {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// Log writes one structured line per request and turns a panic into a JSON
// 500 (logged with the panic value) instead of a dropped connection.
func Log(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w}
			attrs := []any{}
			defer func() {
				if p := recover(); p != nil {
					if sw.status == 0 {
						writeError(sw, http.StatusInternalServerError, "internal server error")
					}
					attrs = append(attrs, "panic", fmt.Sprint(p))
				}
				logger.Info("request", append([]any{
					"method", r.Method, "path", r.URL.Path, "status", sw.status,
					"duration_ms", time.Since(start).Milliseconds(), "ip", clientIP(r),
				}, attrs...)...)
			}()
			next.ServeHTTP(sw, r)
		})
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":%q}`, msg)
}
