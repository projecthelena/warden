package uptime

import (
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
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

	res := runCheck(job, newTestTransport())
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

func newTestTransport() *http.Transport {
	return &http.Transport{DialContext: dialContext}
}
