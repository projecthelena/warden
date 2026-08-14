package uptime

// Checks refuse to connect to link-local addresses.
//
// 169.254.169.254 is where AWS, GCP, Azure, Hetzner and others serve instance metadata,
// including credentials. Nothing worth monitoring lives in that range, and reaching it
// is a way to read secrets out of Warden: a failed check stores up to
// ResponseBodyMaxBytes of whatever the target answered, and setting acceptedStatusCodes
// to something the target never returns makes every response count as failed. Whoever
// can create a monitor could then read the host's credentials from the incident.
//
// Private ranges stay reachable. Monitoring an internal network is the product.
//
// The check runs in the dialer, on the resolved address, which is the only point where
// the real destination is known. Validating the target string instead would miss a
// hostname that resolves there, a name that changes after it was validated, and a
// redirect the target itself returns.

import (
	"context"
	"errors"
	"net"
	"net/http"
	"syscall"
	"time"
)

var errBlockedTarget = errors.New("refusing to connect to a link-local address: that range carries cloud instance metadata, not services worth monitoring")

func blockedAddress(ip net.IP) bool {
	return ip != nil && (ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast())
}

func controlDial(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if blockedAddress(net.ParseIP(host)) {
		return errBlockedTarget
	}
	return nil
}

// safeDialer is for callers that own their timeout, which is every check that dials
// directly rather than through an http.Client.
func safeDialer(timeout time.Duration) *net.Dialer {
	return &net.Dialer{Timeout: timeout, Control: controlDial}
}

// checkTransport is the shared transport every HTTP check runs through. The dialer
// carries no timeout of its own: the per-monitor one lives on the http.Client, and
// duplicating it here would quietly cap a monitor configured to wait longer.
func checkTransport() *http.Transport {
	return &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     30 * time.Second,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return (&net.Dialer{Control: controlDial}).DialContext(ctx, network, address)
		},
	}
}
