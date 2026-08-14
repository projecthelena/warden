package uptime

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
)

func TestProbeTCPUpWhenPortAccepts(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to open listener: %v", err)
	}
	defer func() { _ = ln.Close() }()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	out := probeTCP(ln.Addr().String(), 2*time.Second)
	if !out.up {
		t.Fatalf("expected listening port to be up, got err=%q", out.err)
	}
}

func TestProbeTCPDownWhenNothingListens(t *testing.T) {
	// Grab a port and immediately release it so nothing is listening on it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to open listener: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	out := probeTCP(addr, 2*time.Second)
	if out.up {
		t.Fatal("expected closed port to be down")
	}
	if out.fatal {
		t.Fatal("a closed port is a normal outage, not a fatal target")
	}
}

func TestProbeTCPRejectsMalformedTarget(t *testing.T) {
	for _, target := range []string{"example.com", "http://example.com:80", "example.com:"} {
		out := probeTCP(target, time.Second)
		if out.up {
			t.Fatalf("expected %q to be down", target)
		}
		if !out.fatal {
			t.Fatalf("expected %q to be reported as a fatal target so it is not retried", target)
		}
	}
}

func TestProbeDNSResolvesLocalhost(t *testing.T) {
	// localhost resolves from the hosts file, so this holds with no network.
	out := probeDNS("localhost", nil, 5*time.Second)
	if !out.up {
		t.Fatalf("expected localhost to resolve, got err=%q", out.err)
	}
}

func TestProbeDNSDownWhenResolverIsUnreachable(t *testing.T) {
	// Port 1 on the loopback refuses connections, so the lookup cannot succeed.
	cfg := &db.RequestConfig{DNSResolver: "127.0.0.1:1"}

	out := probeDNS("example.com", cfg, 2*time.Second)
	if out.up {
		t.Fatal("expected an unreachable resolver to report down")
	}
	if out.fatal {
		t.Fatal("an unreachable resolver is a normal outage, not a fatal target")
	}
}

func TestProbeDNSRejectsUnknownRecordType(t *testing.T) {
	cfg := &db.RequestConfig{DNSRecordType: "SRV"}

	out := probeDNS("example.com", cfg, time.Second)
	if out.up {
		t.Fatal("expected an unsupported record type to report down")
	}
	if !out.fatal {
		t.Fatal("an unsupported record type cannot succeed on a retry")
	}
}

func TestValidDNSRecordType(t *testing.T) {
	for _, rt := range []string{"A", "AAAA", "MX", "NS", "TXT"} {
		if !ValidDNSRecordType(rt) {
			t.Errorf("expected %q to be a valid record type", rt)
		}
	}
	for _, rt := range []string{"", "a", "SRV", "ANY"} {
		if ValidDNSRecordType(rt) {
			t.Errorf("expected %q to be rejected", rt)
		}
	}
}

func TestProbePingLoopback(t *testing.T) {
	out := probePing("127.0.0.1", 5*time.Second)
	if out.fatal && strings.Contains(out.err, "icmp socket unavailable") {
		t.Skip("no ICMP socket available in this environment")
	}
	if !out.up {
		t.Fatalf("expected loopback to answer a ping, got err=%q", out.err)
	}
}

func TestProbePingUnresolvableHostIsRetryable(t *testing.T) {
	out := probePing("host.invalid.", 2*time.Second)
	if out.up {
		t.Fatal("expected an unresolvable host to report down")
	}
	if out.fatal {
		t.Fatal("a name that does not resolve is a normal outage, not a fatal target")
	}
}

func TestWithDefaultPort(t *testing.T) {
	cases := map[string]string{
		"1.1.1.1":      "1.1.1.1:53",
		"1.1.1.1:5353": "1.1.1.1:5353",
		"dns.internal": "dns.internal:53",
		"2001:db8::1":  "[2001:db8::1]:53",
	}
	for in, want := range cases {
		if got := withDefaultPort(in, "53"); got != want {
			t.Errorf("withDefaultPort(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRunCheckRoutesByType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	transport := &http.Transport{}

	// An empty type is the pre-existing HTTP behaviour and must keep working.
	res := runCheck(Job{MonitorID: "m1", URL: srv.URL}, transport)
	if !res.Status || res.StatusCode != http.StatusOK {
		t.Fatalf("expected an untyped job to run an HTTP check, got status=%v code=%d err=%q", res.Status, res.StatusCode, res.Error)
	}

	// The same URL probed as TCP is a malformed target, which proves the switch routes.
	res = runCheck(Job{MonitorID: "m1", Type: db.MonitorTypeTCP, URL: srv.URL}, transport)
	if res.Status {
		t.Fatal("expected an URL used as a TCP target to fail")
	}
	if !strings.Contains(res.Error, "expected host:port") {
		t.Fatalf("expected the TCP probe to report the target shape, got %q", res.Error)
	}

	// Stripping the scheme turns it into a valid TCP target for the same server.
	res = runCheck(Job{MonitorID: "m1", Type: db.MonitorTypeTCP, URL: strings.TrimPrefix(srv.URL, "http://")}, transport)
	if !res.Status {
		t.Fatalf("expected the test server port to accept a TCP connection, got err=%q", res.Error)
	}

	res = runCheck(Job{MonitorID: "m1", Type: db.MonitorTypeDNS, URL: "localhost"}, transport)
	if !res.Status {
		t.Fatalf("expected localhost to resolve through the dns branch, got err=%q", res.Error)
	}

	res = runCheck(Job{MonitorID: "m1", Type: db.MonitorTypePing, URL: "not-a-real-host.invalid."}, transport)
	if res.Status {
		t.Fatal("expected an unresolvable host to fail through the ping branch")
	}
}

func TestRunCheckRetriesUntilUp(t *testing.T) {
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	job := Job{
		MonitorID:     "m1",
		URL:           srv.URL,
		RequestConfig: &db.RequestConfig{RetryCount: 3},
	}

	res := runCheck(job, &http.Transport{})
	if !res.Status {
		t.Fatalf("expected the check to recover on the third attempt, got err=%q", res.Error)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestRunCheckDoesNotRetryFatalTargets(t *testing.T) {
	job := Job{
		MonitorID:     "m1",
		Type:          db.MonitorTypeTCP,
		URL:           "not-a-host-port",
		RequestConfig: &db.RequestConfig{RetryCount: 5},
	}

	start := time.Now()
	res := runCheck(job, &http.Transport{})
	elapsed := time.Since(start)

	if res.Status {
		t.Fatal("expected a malformed target to report down")
	}
	// Five retries would sleep for five seconds; a fatal outcome must skip them.
	if elapsed > retryBackoff {
		t.Fatalf("expected no retries for a fatal target, took %s", elapsed)
	}
}

// TestManagerRunsTCPMonitorEndToEnd covers the whole path a non-HTTP monitor takes:
// stored row -> Sync -> scheduled job -> worker -> recorded result.
func TestManagerRunsTCPMonitorEndToEnd(t *testing.T) {
	n := testDBCounter.Add(1)
	store, err := db.NewStore(db.NewTestConfigWithPath(fmt.Sprintf("file:tcp_e2e_%d?mode=memory&cache=shared", n)))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	setIntegrationTestDefaults(store)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to open listener: %v", err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	m := NewManager(store)
	m.Start()
	defer m.Stop()

	if err := store.CreateMonitor(db.Monitor{
		ID:       "m-tcp-e2e",
		Type:     db.MonitorTypeTCP,
		GroupID:  "g-default",
		Name:     "TCP End To End",
		URL:      ln.Addr().String(),
		Active:   true,
		Interval: 1,
	}); err != nil {
		t.Fatalf("failed to create monitor: %v", err)
	}

	m.Sync()

	mon := m.GetMonitor("m-tcp-e2e")
	if mon == nil {
		t.Fatal("expected the manager to schedule the monitor")
	}
	if mon.GetType() != db.MonitorTypeTCP {
		t.Fatalf("expected the scheduled monitor to keep its type, got %q", mon.GetType())
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if isUp, _, hasHistory, _ := mon.GetLastStatus(); hasHistory {
			if !isUp {
				t.Fatal("expected the TCP monitor to record an up check")
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the first TCP check")
}

// The incident drill-down depends on failed HTTP checks carrying what the server
// returned. That capture moved into probeHTTP when the check types were added, so it is
// worth pinning here: on failure it is present, on success it is not.
func TestRunCheckCapturesResponseOnlyOnFailure(t *testing.T) {
	var fail bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Set-Cookie", "session=secret")
		if fail {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"boom"}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	transport := &http.Transport{}

	fail = true
	res := runCheck(Job{MonitorID: "m1", URL: srv.URL}, transport)
	if res.Status {
		t.Fatal("expected a 500 to report down")
	}
	if !strings.Contains(res.ResponseBody, "boom") {
		t.Errorf("expected the failed body to be captured, got %q", res.ResponseBody)
	}
	if !strings.Contains(res.ResponseHeaders, "application/json") {
		t.Errorf("expected the content type to be captured, got %q", res.ResponseHeaders)
	}
	if strings.Contains(strings.ToLower(res.ResponseHeaders), "set-cookie") {
		t.Errorf("sensitive headers must stay out of the capture, got %q", res.ResponseHeaders)
	}

	fail = false
	res = runCheck(Job{MonitorID: "m1", URL: srv.URL}, transport)
	if !res.Status {
		t.Fatalf("expected a 200 to report up, got err=%q", res.Error)
	}
	if res.ResponseBody != "" || res.ResponseHeaders != "" {
		t.Errorf("a passing check must not carry a captured response, got body=%q headers=%q", res.ResponseBody, res.ResponseHeaders)
	}
}

// Non-HTTP checks have no response to capture, so the diagnostic fields stay empty.
func TestRunCheckLeavesResponseEmptyForNonHTTPTypes(t *testing.T) {
	res := runCheck(Job{MonitorID: "m1", Type: db.MonitorTypeTCP, URL: "127.0.0.1:1"}, &http.Transport{})
	if res.Status {
		t.Fatal("expected a closed port to report down")
	}
	if res.ResponseBody != "" || res.ResponseHeaders != "" {
		t.Errorf("a TCP check has no response to capture, got body=%q headers=%q", res.ResponseBody, res.ResponseHeaders)
	}
}

// emptyAnswerDNS serves NOERROR with no records for every query. That is the case a DNS
// monitor has to call down: the name resolved, but it serves nothing.
func emptyAnswerDNS(t *testing.T) string {
	t.Helper()

	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to open dns stub: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	go func() {
		buf := make([]byte, 512)
		for {
			n, addr, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			if n < 12 {
				continue
			}
			reply := make([]byte, n)
			copy(reply, buf[:n])
			reply[2] |= 0x80          // QR: this is a response
			reply[3] &^= 0x0f         // RCODE 0
			reply[6], reply[7] = 0, 0 // no answers
			_, _ = conn.WriteTo(reply, addr)
		}
	}()

	return conn.LocalAddr().String()
}

func TestProbeDNSEmptyAnswerIsDownForEveryRecordType(t *testing.T) {
	resolver := emptyAnswerDNS(t)

	for _, recordType := range []string{"A", "AAAA", "MX", "NS", "TXT"} {
		cfg := &db.RequestConfig{DNSRecordType: recordType, DNSResolver: resolver}

		out := probeDNS("example.com", cfg, 3*time.Second)
		if out.up {
			t.Errorf("%s: expected an empty answer to report down", recordType)
		}
		if out.fatal {
			t.Errorf("%s: an empty answer is a normal outage, not a fatal target", recordType)
		}
	}
}

func TestProbeDNSLowercaseRecordTypeIsAccepted(t *testing.T) {
	out := probeDNS("localhost", &db.RequestConfig{DNSRecordType: "a"}, 5*time.Second)
	if !out.up {
		t.Fatalf("expected a lowercase record type to be normalized, got err=%q", out.err)
	}
}

func TestPingProtoForAddressFamily(t *testing.T) {
	v4 := pingProtoFor(net.ParseIP("127.0.0.1"))
	if v4.unprivilegedNetwork != "udp4" || v4.rawNetwork != "ip4:icmp" {
		t.Errorf("unexpected IPv4 networks: %+v", v4)
	}

	v6 := pingProtoFor(net.ParseIP("::1"))
	if v6.unprivilegedNetwork != "udp6" || v6.rawNetwork != "ip6:ipv6-icmp" {
		t.Errorf("unexpected IPv6 networks: %+v", v6)
	}
	if v4.protocol == v6.protocol {
		t.Error("expected the two families to use different ICMP protocol numbers")
	}
}

// Changing the type has to restart the monitor goroutine, otherwise the running one
// keeps probing the old way until something else happens to restart it.
func TestSyncRestartsMonitorWhenTypeChanges(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfigWithPath(fmt.Sprintf("file:type_change_%d?mode=memory&cache=shared", testDBCounter.Add(1))))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	setIntegrationTestDefaults(store)

	// Reset rather than Stop: this drives Sync without Start, so nothing is draining the
	// job queue and closing it would race the monitor goroutines still scheduling on it.
	m := NewManager(store)
	defer m.Reset()

	if err := store.CreateMonitor(db.Monitor{
		ID:       "m-type-change",
		GroupID:  "g-default",
		Name:     "Type Change",
		URL:      "example.com",
		Active:   true,
		Interval: 60,
	}); err != nil {
		t.Fatalf("failed to create monitor: %v", err)
	}

	m.Sync()
	before := m.GetMonitor("m-type-change")
	if before == nil {
		t.Fatal("expected the monitor to be scheduled")
	}
	if before.GetType() != db.MonitorTypeHTTP {
		t.Fatalf("expected http, got %q", before.GetType())
	}

	if err := store.UpdateMonitor("m-type-change", db.MonitorTypePing, "Type Change", "example.com", 60, nil, nil, nil, nil); err != nil {
		t.Fatalf("failed to update monitor: %v", err)
	}
	m.Sync()

	after := m.GetMonitor("m-type-change")
	if after == nil {
		t.Fatal("expected the monitor to still be scheduled")
	}
	if after == before {
		t.Error("expected a fresh monitor after the type changed, got the same instance")
	}
	if after.GetType() != db.MonitorTypePing {
		t.Errorf("expected ping after the update, got %q", after.GetType())
	}
}

// Every worker runs its own ICMP socket, and on the raw fallback each one sees every
// reply on the host. Concurrent pings to the same target must not steal each other's
// answers, which is what the per-check payload is for.
func TestProbePingConcurrent(t *testing.T) {
	if out := probePing("127.0.0.1", 5*time.Second); out.fatal {
		t.Skipf("no ICMP socket available: %s", out.err)
	}

	const workers = 20
	results := make(chan checkOutcome, workers)
	for i := 0; i < workers; i++ {
		go func() { results <- probePing("127.0.0.1", 5*time.Second) }()
	}

	for i := 0; i < workers; i++ {
		if out := <-results; !out.up {
			t.Errorf("concurrent ping %d reported down: %s", i, out.err)
		}
	}
}

func TestRunCheckMarksUnrunnableChecks(t *testing.T) {
	res := runCheck(Job{MonitorID: "m1", Type: db.MonitorTypeTCP, URL: "not-a-host-port"}, &http.Transport{})
	if !res.NotRun {
		t.Error("expected a target Warden cannot address to be marked as not run")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	// A target that answered badly did run. Only the reason differs.
	res = runCheck(Job{MonitorID: "m1", URL: srv.URL}, &http.Transport{})
	if res.Status {
		t.Fatal("expected a 500 to report down")
	}
	if res.NotRun {
		t.Error("a check that reached the target and got 500 did run")
	}
}

// The ICMP permission error is the one an operator is most likely to hit, and the only
// one they cannot solve from inside Warden. It has to say what to do in plain words.
func TestPingPermissionErrorIsActionable(t *testing.T) {
	out := probePing("127.0.0.1", 2*time.Second)
	if !out.fatal {
		t.Skip("ICMP is permitted here, so the permission error cannot be produced")
	}

	if !strings.Contains(out.err, pingPermissionsDoc) {
		t.Errorf("expected the error to link to the docs, got %q", out.err)
	}
	if !strings.Contains(out.err, "not allowed to send ping") {
		t.Errorf("expected the error to say what happened in plain words, got %q", out.err)
	}
}

func TestPingPermissionsDocIsAFullURL(t *testing.T) {
	// A repo-relative path is useless to someone reading a dashboard or a Slack alert.
	if !strings.HasPrefix(pingPermissionsDoc, "https://") {
		t.Errorf("expected a followable URL, got %q", pingPermissionsDoc)
	}
	if !strings.Contains(pingPermissionsDoc, "#") {
		t.Errorf("expected a link to the section, not the whole page: %q", pingPermissionsDoc)
	}
}

// A check that never ran must not be reported as "Monitor is down": that sends the
// operator looking for a network problem instead of the permission they are missing.
func TestUnrunnableCheckReportsTheReasonNotJustDown(t *testing.T) {
	n := testDBCounter.Add(1)
	store, err := db.NewStore(db.NewTestConfigWithPath(fmt.Sprintf("file:notrun_%d?mode=memory&cache=shared", n)))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	setIntegrationTestDefaults(store)

	m := NewManager(store)
	m.Start()
	defer m.Stop()

	if err := store.CreateMonitor(db.Monitor{
		ID:       "m-notrun",
		Type:     db.MonitorTypeTCP,
		GroupID:  "g-default",
		Name:     "Unreachable Config",
		URL:      "missing-the-port",
		Active:   true,
		Interval: 1,
	}); err != nil {
		t.Fatalf("failed to create monitor: %v", err)
	}
	m.Sync()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		events, err := store.GetMonitorEvents("m-notrun", 5)
		if err == nil && len(events) > 0 {
			if events[0].Message == "Monitor is down" {
				t.Fatalf("expected the reason to replace the generic message, got %q", events[0].Message)
			}
			if !strings.Contains(events[0].Message, "expected host:port") {
				t.Fatalf("expected the message to carry the reason, got %q", events[0].Message)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the down event")
}
