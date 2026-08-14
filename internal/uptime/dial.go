package uptime

import (
	"context"
	"fmt"
	"net"
	"syscall"
	"time"
)

// Monitoring a private network is the product, so private ranges stay reachable. The
// link-local range is different: 169.254.169.254 is where every cloud provider serves
// instance credentials, and nobody points an uptime monitor at that on purpose.
//
// Without this an operator who can create monitors can read the host's cloud
// credentials, because a failed check stores up to ResponseBodyMaxBytes of whatever the
// target answered and anyone can read that back. Setting acceptedStatusCodes to
// something the metadata service never returns is enough to make any response "failed".
//
// The check runs on the resolved address rather than on the target string, so a hostname
// that resolves to the metadata address is refused too, whether it was written that way
// on purpose or the name changed after it was validated.
func blockedAddress(ip net.IP) bool {
	return ip != nil && (ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast())
}

var errBlockedTarget = fmt.Errorf("refusing to connect to a link-local address: that range carries cloud instance metadata, not services worth monitoring")

// controlDial refuses the connection after the address is resolved and before it is
// made, which is the only point where the real destination is known.
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

// safeDialer is the dialer every check uses, so no probe can reach the metadata service.
func safeDialer(timeout time.Duration) *net.Dialer {
	return &net.Dialer{Timeout: timeout, Control: controlDial}
}

// dialContext is the hook for the shared HTTP transport.
func dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return safeDialer(defaultCheckTimeout).DialContext(ctx, network, address)
}
