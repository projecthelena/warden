// Command faketarget runs a small local HTTP server that returns deterministic or random
// failures, so Warden monitors can be pointed at it to exercise the enriched event capture
// path (status code, latency, error message, response body excerpt, response headers).
//
// Routes:
//
//	GET /healthy           — always 200 OK with a JSON body
//	GET /down              — always 503, HTML error page
//	GET /error             — always 500, JSON error payload
//	GET /slow              — 200 OK but sleeps `latencyMs` (default 1500ms) to trip degraded
//	GET /flaky?fail=N      — fails ~N% of the time (default 50). Otherwise 200.
//	GET /timeout           — sleeps 30s; intended for monitors with a short timeout
//	GET /status/{code}     — returns the given numeric status with a synthetic body
//
// Run:
//
//	go run ./cmd/faketarget -listen :8888
//
// Then in Warden create a monitor pointing at e.g. http://localhost:8888/down
// with a short interval (10s) and watch the events appear on /monitors/:id with
// status code, headers and a body excerpt to expand.
package main

import (
	crand "crypto/rand"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// safeLog strips control characters from request-derived strings before they reach the log,
// so a crafted path can't forge log lines. Mirrors internal/api.sanitizeLog.
func safeLog(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r < 0x20 || r == 0x7F {
			continue
		}
		b.WriteRune(r)
	}
	s = b.String()
	if len(s) > 128 {
		s = s[:128] + "..."
	}
	return s
}

func main() {
	listen := flag.String("listen", ":8888", "address to listen on")
	latencyMs := flag.Int("slow-latency-ms", 1500, "latency for /slow")
	flag.Parse()

	mux := http.NewServeMux()

	mux.HandleFunc("/healthy", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("→ %s %s", safeLog(r.Method), safeLog(r.URL.Path))
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Faketarget", "healthy")
		_, _ = fmt.Fprintln(w, `{"ok":true,"service":"faketarget"}`)
	})

	mux.HandleFunc("/down", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("→ %s %s [503]", safeLog(r.Method), safeLog(r.URL.Path))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Server", "faketarget/1.0")
		w.Header().Set("X-Request-Id", randomID())
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprintln(w, `<!doctype html><html><head><title>503</title></head><body><h1>Service Unavailable</h1><p>Backend overloaded — pretend upstream returned a long HTML error page so you can see the body excerpt in Warden.</p><pre>trace_id=abc-123\nupstream=db-primary\nattempt=3</pre></body></html>`)
	})

	mux.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("→ %s %s [500]", safeLog(r.Method), safeLog(r.URL.Path))
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", randomID())
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintln(w, `{"error":"internal_error","message":"database connection refused","traceId":"`+randomID()+`","upstream":"db-primary","attempt":3}`)
	})

	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("→ %s %s [slow %dms]", safeLog(r.Method), safeLog(r.URL.Path), *latencyMs)
		time.Sleep(time.Duration(*latencyMs) * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintln(w, `{"ok":true,"slowedBy":"`+strconv.Itoa(*latencyMs)+`ms"}`)
	})

	mux.HandleFunc("/flaky", func(w http.ResponseWriter, r *http.Request) {
		failPct := 50
		if v := r.URL.Query().Get("fail"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 100 {
				failPct = n
			}
		}
		if rand.Intn(100) < failPct { // #nosec G404 -- simulating flakiness, not a security decision
			log.Printf("→ %s %s [flaky FAIL]", safeLog(r.Method), safeLog(r.URL.Path))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = fmt.Fprintln(w, `{"error":"bad_gateway","message":"upstream timed out","upstream":"api-backend"}`)
			return
		}
		log.Printf("→ %s %s [flaky ok]", safeLog(r.Method), safeLog(r.URL.Path))
		_, _ = fmt.Fprintln(w, `{"ok":true}`)
	})

	mux.HandleFunc("/timeout", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("→ %s %s [sleeping 30s]", safeLog(r.Method), safeLog(r.URL.Path))
		time.Sleep(30 * time.Second)
		_, _ = fmt.Fprintln(w, "late response")
	})

	mux.HandleFunc("/status/", func(w http.ResponseWriter, r *http.Request) {
		codeStr := strings.TrimPrefix(r.URL.Path, "/status/")
		code, err := strconv.Atoi(codeStr)
		if err != nil || code < 100 || code > 599 {
			http.Error(w, "invalid status code", http.StatusBadRequest)
			return
		}
		log.Printf("→ %s %s [%d]", safeLog(r.Method), safeLog(r.URL.Path), code)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", randomID())
		w.WriteHeader(code)
		// Echo back only the parsed code, never the raw path — reflecting request input
		// into the response body is what makes a test server a stored-XSS foot-gun.
		_, _ = fmt.Fprintf(w, `{"status":%d,"ts":"%s"}`+"\n", code, time.Now().UTC().Format(time.RFC3339))
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprintln(w, "faketarget — routes: /healthy /down /error /slow /flaky?fail=N /timeout /status/{code}")
	})

	log.Printf("faketarget listening on %s (slow latency=%dms)", *listen, *latencyMs)
	// No WriteTimeout on purpose: /timeout sleeps 30s and needs to hold the connection open.
	srv := &http.Server{
		Addr:              *listen,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func randomID() string {
	const chars = "abcdef0123456789"
	b := make([]byte, 16)
	if _, err := crand.Read(b); err != nil {
		return "0000000000000000"
	}
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b)
}
