package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

var ok = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) })

func call(h http.Handler, method, remote string, header map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "/x", nil)
	r.RemoteAddr = remote
	for k, v := range header {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestRateLimitIsPerIP(t *testing.T) {
	h := RateLimit(rate.Every(time.Hour), 2)(ok)
	for i := range 2 {
		if w := call(h, "POST", "10.0.0.1:1111", nil); w.Code != http.StatusTeapot {
			t.Fatalf("request %d = %d", i, w.Code)
		}
	}
	w := call(h, "POST", "10.0.0.1:2222", nil) // same IP, different port
	if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "" || w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("over limit = %d %v", w.Code, w.Header())
	}
	if w := call(h, "POST", "10.0.0.2:1111", nil); w.Code != http.StatusTeapot {
		t.Fatalf("other IP = %d", w.Code)
	}
}

func TestRateLimitForgetsIdleIPs(t *testing.T) {
	l := newLimiter(rate.Every(time.Hour), 1)
	now := time.Now()
	l.allow("10.0.0.1", now)
	l.allow("10.0.0.2", now.Add(idleAfter+time.Minute)) // triggers a sweep
	if _, kept := l.clients["10.0.0.1"]; kept || len(l.clients) != 1 {
		t.Fatalf("clients after sweep = %v", l.clients)
	}
}

func TestCORS(t *testing.T) {
	h := CORS([]string{"http://localhost:5173"})(ok)

	w := call(h, "OPTIONS", "1.1.1.1:1", map[string]string{"Origin": "http://localhost:5173", "Access-Control-Request-Method": "POST"})
	if w.Code != http.StatusNoContent || w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" ||
		w.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Fatalf("preflight = %d %v", w.Code, w.Header())
	}
	if w := call(h, "GET", "1.1.1.1:1", map[string]string{"Origin": "http://localhost:5173"}); w.Code != http.StatusTeapot ||
		w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatalf("allowed GET = %d %v", w.Code, w.Header())
	}
	if w := call(h, "GET", "1.1.1.1:1", map[string]string{"Origin": "https://evil.example"}); w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("foreign origin allowed: %v", w.Header())
	}
}

func TestLogRecordsRequestsAndRecoversPanics(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	h := Log(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { panic("boom") }))

	w := call(h, "GET", "10.0.0.9:5555", nil)
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if w.Code != http.StatusInternalServerError || body["error"] != "internal server error" {
		t.Fatalf("panic response = %d %q", w.Code, w.Body.String())
	}
	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("log is not one JSON line: %q", buf.String())
	}
	if line["method"] != "GET" || line["path"] != "/x" || line["status"] != float64(500) || line["ip"] != "10.0.0.9" || line["panic"] != "boom" {
		t.Fatalf("log line = %v", line)
	}
}

func TestClientIPTrustsRealIPOnlyBehindProxy(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "172.18.0.5:41000" // the reverse proxy
	r.Header.Set("X-Real-IP", "203.0.113.9")

	if got := clientIP(r); got != "172.18.0.5" {
		t.Fatalf("without TrustProxy, clientIP = %s; a client must not pick its own IP", got)
	}
	TrustProxy = true
	defer func() { TrustProxy = false }()
	if got := clientIP(r); got != "203.0.113.9" {
		t.Fatalf("behind proxy, clientIP = %s, want the X-Real-IP", got)
	}
	r.Header.Del("X-Real-IP")
	if got := clientIP(r); got != "172.18.0.5" {
		t.Fatalf("missing header, clientIP = %s, want RemoteAddr", got)
	}
}
