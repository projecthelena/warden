package uptime

// Checks refuse to connect to link-local addresses, where AWS, GCP, Azure and Hetzner
// all serve instance credentials at 169.254.169.254.
//
// Reaching that range reads secrets out of Warden: a failed check stores what the target
// answered, and acceptedStatusCodes can make any response count as failed, so whoever
// creates a monitor could read the host's credentials from the incident.
//
// Private ranges stay reachable, because monitoring an internal network is the product.
// The check runs in the dialer on the resolved address, which also covers a hostname
// that points there, a name that changes after validation, and a redirect from the
// target itself.

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
