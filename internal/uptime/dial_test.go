package uptime

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
)

func TestBlockedAddress(t *testing.T) {
	blocked := []string{
		"169.254.169.254", // AWS, GCP, Azure, Hetzner instance metadata
		"169.254.1.1",
		"fe80::1", // IPv6 link-local
	}
	for _, ip := range blocked {
		if !blockedAddress(net.ParseIP(ip)) {
			t.Errorf("expected %s to be refused", ip)
		}
	}

	// Monitoring a private network is the product; only link-local is out.
	allowed := []string{"127.0.0.1", "10.0.0.5", "192.168.1.1", "172.16.0.1", "8.8.8.8", "::1", "2001:db8::1"}
	for _, ip := range allowed {
		if blockedAddress(net.ParseIP(ip)) {
			t.Errorf("expected %s to be allowed", ip)
		}
	}
}

// The check has to happen on the resolved address, so a hostname pointing at the
// metadata service is refused just the same.
func TestDialRefusesLinkLocal(t *testing.T) {
	_, err := safeDialer(2*time.Second).Dial("tcp", "169.254.169.254:80")
	if err == nil {
		t.Fatal("expected the dial to be refused")
	}
	if !strings.Contains(err.Error(), "link-local") {
		t.Errorf("expected the error to say why, got %q", err)
	}
}

func TestDialAllowsOrdinaryTargets(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to open listener: %v", err)
	}
	defer func() { _ = ln.Close() }()

	conn, err := safeDialer(2*time.Second).Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("expected a normal target to connect: %v", err)
	}
	_ = conn.Close()
}

// The whole point: a check aimed at the metadata service must not come back with a body
// to store, since that is what turns this into credential theft.
func TestProbeHTTPCannotReachMetadata(t *testing.T) {
	job := Job{MonitorID: "m1", URL: "http://169.254.169.254/latest/meta-data/"}

	res := runCheck(job, checkTransport())
	if res.Status {
		t.Fatal("expected the check to fail")
	}
	if res.ResponseBody != "" {
		t.Errorf("expected no body to be captured, got %q", res.ResponseBody)
	}
	if !strings.Contains(res.Error, "link-local") {
		t.Errorf("expected the error to name the reason, got %q", res.Error)
	}
}

// A target Warden was pointed at can answer with a redirect, and the client follows it.
// The block has to survive that, which it does by living in the dialer rather than in
// target validation: the redirect makes a second connection through the same transport.
func TestRedirectToMetadataIsRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
	}))
	defer srv.Close()

	res := runCheck(Job{MonitorID: "m1", URL: srv.URL}, checkTransport())
	if res.Status {
		t.Fatal("expected a redirect into link-local to fail the check")
	}
	if res.ResponseBody != "" {
		t.Errorf("expected nothing to be captured, got %q", res.ResponseBody)
	}
	if !strings.Contains(res.Error, "link-local") {
		t.Errorf("expected the link-local refusal, got %q", res.Error)
	}
}

// The resolver a DNS monitor is pointed at is dialed too, so it cannot be used to reach
// the same place.
func TestDNSResolverCannotBeLinkLocal(t *testing.T) {
	out := probeDNS("example.com", &db.RequestConfig{DNSResolver: "169.254.169.254"}, 2*time.Second)
	if out.up {
		t.Fatal("expected the lookup to fail")
	}
	if !strings.Contains(out.err, "link-local") {
		t.Errorf("expected the link-local refusal, got %q", out.err)
	}
}

// The per-monitor timeout lives on the http.Client. A timeout on the transport's dialer
// as well would quietly cap a monitor configured to wait longer, so the shared transport
// must not carry one.
func TestCheckTransportDoesNotCapTheMonitorsTimeout(t *testing.T) {
	// A listener that accepts and then says nothing: the check has to be cut short by
	// its own timeout, not by something shorter hidden in the transport.
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
			// Hold it open without answering.
			go func() { time.Sleep(10 * time.Second); _ = conn.Close() }()
		}
	}()

	job := Job{
		MonitorID:     "m1",
		URL:           "http://" + ln.Addr().String(),
		RequestConfig: &db.RequestConfig{TimeoutSeconds: 3},
	}

	start := time.Now()
	res := runCheck(job, checkTransport())
	elapsed := time.Since(start)

	if res.Status {
		t.Fatal("expected the check to time out")
	}
	// Would land near 5s if the transport carried defaultCheckTimeout instead.
	if elapsed < 2500*time.Millisecond || elapsed > 4500*time.Millisecond {
		t.Errorf("expected the monitor's own 3s timeout to govern, took %s", elapsed)
	}
}

// A TCP monitor leaks no body, but it would still make Warden a port scanner of the
// metadata service, and the block should not depend on which check type is used.
func TestProbeTCPCannotReachLinkLocal(t *testing.T) {
	out := probeTCP("169.254.169.254:80", 2*time.Second)
	if out.up {
		t.Fatal("expected the check to fail")
	}
	if !strings.Contains(out.err, "link-local") {
		t.Errorf("expected the link-local refusal, got %q", out.err)
	}
}
