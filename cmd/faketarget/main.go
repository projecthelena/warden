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
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func main() {
	listen := flag.String("listen", ":8888", "address to listen on")
	latencyMs := flag.Int("slow-latency-ms", 1500, "latency for /slow")
	flag.Parse()

	mux := http.NewServeMux()

	mux.HandleFunc("/healthy", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("→ %s %s", r.Method, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Faketarget", "healthy")
		_, _ = fmt.Fprintln(w, `{"ok":true,"service":"faketarget"}`)
	})

	mux.HandleFunc("/down", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("→ %s %s [503]", r.Method, r.URL.Path)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Server", "faketarget/1.0")
		w.Header().Set("X-Request-Id", randomID())
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprintln(w, `<!doctype html><html><head><title>503</title></head><body><h1>Service Unavailable</h1><p>Backend overloaded — pretend upstream returned a long HTML error page so you can see the body excerpt in Warden.</p><pre>trace_id=abc-123\nupstream=db-primary\nattempt=3</pre></body></html>`)
	})

	mux.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("→ %s %s [500]", r.Method, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", randomID())
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintln(w, `{"error":"internal_error","message":"database connection refused","traceId":"`+randomID()+`","upstream":"db-primary","attempt":3}`)
	})

	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("→ %s %s [slow %dms]", r.Method, r.URL.Path, *latencyMs)
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
		if rand.Intn(100) < failPct {
			log.Printf("→ %s %s [flaky FAIL]", r.Method, r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = fmt.Fprintln(w, `{"error":"bad_gateway","message":"upstream timed out","upstream":"api-backend"}`)
			return
		}
		log.Printf("→ %s %s [flaky ok]", r.Method, r.URL.Path)
		_, _ = fmt.Fprintln(w, `{"ok":true}`)
	})

	mux.HandleFunc("/timeout", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("→ %s %s [sleeping 30s]", r.Method, r.URL.Path)
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
		log.Printf("→ %s %s [%d]", r.Method, r.URL.Path, code)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", randomID())
		w.WriteHeader(code)
		_, _ = fmt.Fprintf(w, `{"status":%d,"path":"%s","ts":"%s"}`+"\n", code, r.URL.Path, time.Now().UTC().Format(time.RFC3339))
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprintln(w, "faketarget — routes: /healthy /down /error /slow /flaky?fail=N /timeout /status/{code}")
	})

	log.Printf("faketarget listening on %s (slow latency=%dms)", *listen, *latencyMs)
	if err := http.ListenAndServe(*listen, mux); err != nil {
		log.Fatal(err)
	}
}

func randomID() string {
	const chars = "abcdef0123456789"
	b := make([]byte, 16)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}
