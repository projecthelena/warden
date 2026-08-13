package uptime

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"

	"github.com/projecthelena/warden/internal/db"
)

const (
	defaultCheckTimeout = 5 * time.Second
	retryBackoff        = 1 * time.Second
)

// checkOutcome is what a single probe attempt reports back. Only probeHTTP fills
// statusCode, certExpiry and the captured response; the other types leave them zero.
type checkOutcome struct {
	up          bool
	err         string
	statusCode  int
	certExpiry  *time.Time
	respBody    string
	respHeaders string

	// fatal marks a target that cannot be probed at all — a malformed address, or an
	// ICMP socket the process isn't allowed to open. Retrying that only delays the
	// result, so runCheck gives up immediately.
	fatal bool
}

// runCheck reports the outcome of the last attempt. Timing, retries and the result
// envelope are the same for every check type; only the probe in the middle differs.
func runCheck(job Job, transport *http.Transport) CheckResult {
	cfg := job.RequestConfig

	timeout := defaultCheckTimeout
	if cfg != nil && cfg.TimeoutSeconds > 0 {
		timeout = time.Duration(cfg.TimeoutSeconds) * time.Second
	}
	retryCount := 0
	if cfg != nil && cfg.RetryCount > 0 {
		retryCount = cfg.RetryCount
	}

	var (
		out     checkOutcome
		start   time.Time
		latency int64
	)

	for attempt := 0; attempt <= retryCount; attempt++ {
		if attempt > 0 {
			time.Sleep(retryBackoff)
		}

		start = time.Now().UTC()
		out = probe(job, timeout, transport)
		latency = time.Since(start).Milliseconds()

		if out.up || out.fatal {
			break
		}
	}

	return CheckResult{
		MonitorID:       job.MonitorID,
		URL:             job.URL,
		Status:          out.up,
		Latency:         latency,
		Timestamp:       start,
		StatusCode:      out.statusCode,
		Error:           out.err,
		CertExpiry:      out.certExpiry,
		ResponseBody:    out.respBody,
		ResponseHeaders: out.respHeaders,
	}
}

func probe(job Job, timeout time.Duration, transport *http.Transport) checkOutcome {
	switch db.NormalizeMonitorType(job.Type) {
	case db.MonitorTypeTCP:
		return probeTCP(job.URL, timeout)
	case db.MonitorTypePing:
		return probePing(job.URL, timeout)
	case db.MonitorTypeDNS:
		return probeDNS(job.URL, job.RequestConfig, timeout)
	default:
		return probeHTTP(job, timeout, transport)
	}
}

// probeHTTP decides on the status code, and picks up the TLS certificate expiry on the
// way so the SSL warnings have something to work with.
func probeHTTP(job Job, timeout time.Duration, transport *http.Transport) checkOutcome {
	cfg := job.RequestConfig

	method := http.MethodGet
	if cfg != nil && cfg.Method != "" {
		method = cfg.Method
	}

	client := &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
	if cfg != nil && cfg.FollowRedirects != nil && !*cfg.FollowRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	var body io.Reader
	if cfg != nil && cfg.Body != "" {
		body = strings.NewReader(cfg.Body)
	}

	req, err := http.NewRequest(method, job.URL, body)
	if err != nil {
		return checkOutcome{err: err.Error(), fatal: true}
	}
	if cfg != nil {
		for k, v := range cfg.Headers {
			req.Header.Set(k, v)
		}
	}

	resp, err := client.Do(req) // #nosec G704 -- probing an operator-configured target URL is the product
	if err != nil {
		return checkOutcome{err: err.Error()}
	}
	defer func() { _ = resp.Body.Close() }()

	out := checkOutcome{up: true, statusCode: resp.StatusCode}
	if cfg != nil && cfg.AcceptedStatusCodes != "" {
		out.up = isAcceptedStatus(resp.StatusCode, cfg.AcceptedStatusCodes)
	} else if resp.StatusCode >= 400 {
		out.up = false
	}

	// Capture diagnostics on failed checks so the daily digest drill-down can show what
	// the server actually returned.
	if !out.up {
		out.respBody = readLimitedBody(resp.Body)
		out.respHeaders = encodeAllowedHeaders(resp.Header)
	}

	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		notAfter := resp.TLS.PeerCertificates[0].NotAfter
		out.certExpiry = &notAfter
	}

	return out
}

// probeTCP writes nothing to the connection — a service that accepts is a service that
// is listening.
func probeTCP(target string, timeout time.Duration) checkOutcome {
	if _, port, err := net.SplitHostPort(target); err != nil || port == "" {
		return checkOutcome{err: fmt.Sprintf("invalid tcp target %q: expected host:port", target), fatal: true}
	}

	conn, err := net.DialTimeout("tcp", target, timeout)
	if err != nil {
		return checkOutcome{err: err.Error()}
	}
	_ = conn.Close()

	return checkOutcome{up: true}
}

// ValidDNSRecordType reports whether t is a record type a DNS monitor can query.
func ValidDNSRecordType(t string) bool {
	switch t {
	case "A", "AAAA", "CNAME", "MX", "NS", "TXT":
		return true
	}
	return false
}

// probeDNS resolves the target and reports it up when the lookup returns at least one
// record. An empty answer counts as down: the name resolved but serves nothing, which
// is the failure a DNS monitor exists to catch.
func probeDNS(target string, cfg *db.RequestConfig, timeout time.Duration) checkOutcome {
	recordType := "A"
	resolverAddr := ""
	if cfg != nil {
		if cfg.DNSRecordType != "" {
			recordType = strings.ToUpper(cfg.DNSRecordType)
		}
		resolverAddr = cfg.DNSResolver
	}
	if !ValidDNSRecordType(recordType) {
		return checkOutcome{err: fmt.Sprintf("unsupported dns record type %q", recordType), fatal: true}
	}

	resolver := net.DefaultResolver
	if resolverAddr != "" {
		addr := withDefaultPort(resolverAddr, "53")
		resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				d := net.Dialer{Timeout: timeout}
				return d.DialContext(ctx, network, addr)
			},
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	found, err := countDNSRecords(ctx, resolver, recordType, target)
	if err != nil {
		return checkOutcome{err: err.Error()}
	}
	if found == 0 {
		return checkOutcome{err: fmt.Sprintf("no %s records for %s", recordType, target)}
	}

	return checkOutcome{up: true}
}

func countDNSRecords(ctx context.Context, r *net.Resolver, recordType, target string) (int, error) {
	switch recordType {
	case "A":
		addrs, err := r.LookupIP(ctx, "ip4", target)
		return len(addrs), err
	case "AAAA":
		addrs, err := r.LookupIP(ctx, "ip6", target)
		return len(addrs), err
	case "CNAME":
		name, err := r.LookupCNAME(ctx, target)
		if err != nil || name == "" {
			return 0, err
		}
		return 1, nil
	case "MX":
		records, err := r.LookupMX(ctx, target)
		return len(records), err
	case "NS":
		records, err := r.LookupNS(ctx, target)
		return len(records), err
	case "TXT":
		records, err := r.LookupTXT(ctx, target)
		return len(records), err
	}
	return 0, fmt.Errorf("unsupported dns record type %q", recordType)
}

// withDefaultPort appends port to addr when addr carries none, wrapping bare IPv6
// literals in brackets on the way.
func withDefaultPort(addr, port string) string {
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return addr
	}
	return net.JoinHostPort(addr, port)
}

// pingProto carries the address-family specific halves of an ICMP echo exchange.
type pingProto struct {
	unprivilegedNetwork string
	rawNetwork          string
	listenAddr          string
	protocol            int
	echo                icmp.Type
	echoReply           icmp.Type
}

func pingProtoFor(ip net.IP) pingProto {
	if ip.To4() != nil {
		return pingProto{
			unprivilegedNetwork: "udp4",
			rawNetwork:          "ip4:icmp",
			listenAddr:          "0.0.0.0",
			protocol:            ipv4.ICMPTypeEchoReply.Protocol(),
			echo:                ipv4.ICMPTypeEcho,
			echoReply:           ipv4.ICMPTypeEchoReply,
		}
	}
	return pingProto{
		unprivilegedNetwork: "udp6",
		rawNetwork:          "ip6:ipv6-icmp",
		listenAddr:          "::",
		protocol:            ipv6.ICMPTypeEchoReply.Protocol(),
		echo:                ipv6.ICMPTypeEchoRequest,
		echoReply:           ipv6.ICMPTypeEchoReply,
	}
}

// A raw ICMP socket is handed a copy of every ICMP packet the host receives, so each
// check tags its echo with a payload no other worker is using and matches on that.
var pingSeq atomic.Uint64

// probePing prefers an unprivileged datagram socket, which needs no capabilities but
// does need the host to allow it (net.ipv4.ping_group_range on Linux), and falls back to
// a raw socket, which needs root or CAP_NET_RAW. docs/monitor-types.md covers both.
func probePing(target string, timeout time.Duration) checkOutcome {
	ip, err := net.ResolveIPAddr("ip", target)
	if err != nil {
		return checkOutcome{err: fmt.Sprintf("resolve %s: %v", target, err)}
	}

	p := pingProtoFor(ip.IP)

	privileged := false
	conn, err := icmp.ListenPacket(p.unprivilegedNetwork, p.listenAddr)
	if err != nil {
		conn, err = icmp.ListenPacket(p.rawNetwork, p.listenAddr)
		if err != nil {
			return checkOutcome{
				err:   fmt.Sprintf("icmp socket unavailable: %v (see docs/monitor-types.md)", err),
				fatal: true,
			}
		}
		privileged = true
	}
	defer func() { _ = conn.Close() }()

	seq := pingSeq.Add(1)
	payload := fmt.Appendf(nil, "warden-ping-%d", seq)
	wire, err := (&icmp.Message{
		Type: p.echo,
		Code: 0,
		Body: &icmp.Echo{
			ID:   int(seq & 0xffff),
			Seq:  1,
			Data: payload,
		},
	}).Marshal(nil)
	if err != nil {
		return checkOutcome{err: err.Error(), fatal: true}
	}

	// The unprivileged socket speaks UDP addresses even though it carries ICMP.
	var dst net.Addr = &net.IPAddr{IP: ip.IP}
	if !privileged {
		dst = &net.UDPAddr{IP: ip.IP}
	}

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return checkOutcome{err: err.Error()}
	}
	if _, err := conn.WriteTo(wire, dst); err != nil {
		return checkOutcome{err: err.Error()}
	}

	buf := make([]byte, 1500)
	for {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			// Deadline exceeded means no reply came back, which is the target being down.
			return checkOutcome{err: err.Error()}
		}
		reply, err := icmp.ParseMessage(p.protocol, buf[:n])
		if err != nil || reply.Type != p.echoReply {
			continue
		}
		echo, ok := reply.Body.(*icmp.Echo)
		if !ok || !bytes.Equal(echo.Data, payload) {
			continue
		}
		return checkOutcome{up: true}
	}
}
