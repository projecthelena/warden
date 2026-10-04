package uptime

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"

	"github.com/projecthelena/warden/internal/db"
)

func automaticRetryEnabled(job Job) bool {
	cfg := job.RequestConfig
	return cfg != nil && cfg.AutoRetry && db.NormalizeMonitorType(job.Type) == db.MonitorTypeHTTP && cfg.Body == "" && (cfg.Method == "" || cfg.Method == http.MethodGet || cfg.Method == http.MethodHead)
}

func transientHTTPFailure(out checkOutcome) bool {
	if out.up || out.fatal || out.retryAfter {
		return false
	}
	if out.statusCode > 0 {
		switch out.statusCode {
		case http.StatusRequestTimeout, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return true
		}
		return false
	}
	if errors.Is(out.cause, errBlockedTarget) {
		return false
	}
	var certificate *tls.CertificateVerificationError
	if errors.As(out.cause, &certificate) {
		return false
	}
	var dns *net.DNSError
	if errors.As(out.cause, &dns) {
		return !dns.IsNotFound && (dns.IsTimeout || dns.IsTemporary)
	}
	var network *net.OpError
	return errors.As(out.cause, &network) || errors.Is(out.cause, context.DeadlineExceeded) || errors.Is(out.cause, io.EOF) || errors.Is(out.cause, io.ErrUnexpectedEOF)
}
