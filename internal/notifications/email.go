package notifications

// Email is the one channel where the recipient does not control the transport: Slack and
// webhooks are one HTTPS POST to a URL the operator pasted, while SMTP is a stateful
// conversation with a server that may or may not offer encryption, may or may not want
// credentials, and can hang halfway through. Everything unusual in this file comes from
// that difference.

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

const (
	// A hung SMTP server must not stall the notification worker, which drains the queue
	// one event at a time. Same reasoning as the 10s timeout on the webhook client.
	smtpDialTimeout  = 10 * time.Second
	smtpTotalTimeout = 30 * time.Second

	// implicitTLSPort speaks TLS from the first byte (SMTPS). Every other port starts in
	// plaintext and upgrades with STARTTLS.
	implicitTLSPort = 465
)

// EmailNotifier delivers alerts over SMTP.
type EmailNotifier struct {
	config map[string]interface{}
}

func NewEmailNotifier(configJSON string) *EmailNotifier {
	var config map[string]interface{}
	_ = json.Unmarshal([]byte(configJSON), &config)
	return &EmailNotifier{config: config}
}

// emailConfig is the validated form of a channel's config. Parsing it is separate from
// sending so that both the alert path and the digest path fail the same way, and so the
// tests can check the validation without opening a socket.
type emailConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	To       []string
}

func (n *EmailNotifier) parseConfig() (*emailConfig, error) {
	cfg := &emailConfig{
		Host:     configString(n.config, "host"),
		Username: configString(n.config, "username"),
		Password: configString(n.config, "password"),
		From:     configString(n.config, "from"),
	}

	if cfg.Host == "" {
		return nil, fmt.Errorf("host is required")
	}

	port := configString(n.config, "port")
	if port == "" {
		// 587 is the submission port. Defaulting to 25 would pick the server-to-server
		// port, which most providers refuse from an application.
		cfg.Port = 587
	} else {
		p, err := strconv.Atoi(port)
		if err != nil || p < 1 || p > 65535 {
			return nil, fmt.Errorf("port %q is not a valid port number", port)
		}
		cfg.Port = p
	}

	if cfg.From == "" {
		return nil, fmt.Errorf("from address is required")
	}
	if _, err := mail.ParseAddress(cfg.From); err != nil {
		return nil, fmt.Errorf("from address %q is not a valid email address", cfg.From)
	}

	recipients, err := parseRecipients(configString(n.config, "to"))
	if err != nil {
		return nil, err
	}
	cfg.To = recipients

	return cfg, nil
}

// configString reads a config value as a string. Ports arrive as a JSON number from the
// API and as a string from a hand-written config, so both are accepted.
func configString(config map[string]interface{}, key string) string {
	switch v := config[key].(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return ""
	}
}

// parseRecipients splits and validates a comma-separated recipient list.
func parseRecipients(raw string) ([]string, error) {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		addr := strings.TrimSpace(part)
		if addr == "" {
			continue
		}
		parsed, err := mail.ParseAddress(addr)
		if err != nil {
			return nil, fmt.Errorf("recipient %q is not a valid email address", addr)
		}
		out = append(out, parsed.Address)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("at least one recipient is required")
	}
	if len(out) > 50 {
		return nil, fmt.Errorf("too many recipients (max 50)")
	}
	return out, nil
}

// ValidateEmailConfig reports whether a channel config is complete enough to send with.
// The API layer calls it so that a broken channel is rejected on the form rather than
// discovered on the night something goes down.
func ValidateEmailConfig(configJSON string) error {
	_, err := NewEmailNotifier(configJSON).parseConfig()
	return err
}

func (n *EmailNotifier) Send(event NotificationEvent) error {
	cfg, err := n.parseConfig()
	if err != nil {
		return err
	}

	subject := fmt.Sprintf("[Warden] %s: %s", eventTitle(event.Type), event.MonitorName)
	body := eventBody{
		Title:   eventTitle(event.Type),
		Color:   eventColor(event.Type),
		Monitor: event.MonitorName,
		URL:     event.MonitorURL,
		Message: event.Message,
		Time:    event.Time,
	}

	msg, err := buildMessage(cfg, subject, body.text(), body.html())
	if err != nil {
		return err
	}
	return deliver(cfg, msg)
}

func (n *EmailNotifier) sendDigest(summary digestSummary) error {
	cfg, err := n.parseConfig()
	if err != nil {
		return err
	}

	dateStr := summary.Date.Format("January 2, 2006")
	subject := fmt.Sprintf("[Warden] Daily Summary — %s", dateStr)
	if summary.TotalEvents == 0 {
		subject = fmt.Sprintf("[Warden] Daily Summary — all systems operational (%s)", dateStr)
	}

	msg, err := buildMessage(cfg, subject, digestText(summary), digestHTML(summary))
	if err != nil {
		return err
	}
	return deliver(cfg, msg)
}

// deliver opens the SMTP conversation and hands over one message.
//
// The TLS rule: port 465 is TLS from the first byte, anything else upgrades with STARTTLS
// when the server offers it. When it doesn't, the message still goes out unencrypted only
// if there are no credentials to leak — a local relay with no auth is a normal
// self-hosted setup. If credentials exist, net/smtp refuses to send them over a plaintext
// link anyway, and we surface that as an error instead of silently not authenticating.
func deliver(cfg *emailConfig, msg []byte) error {
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))

	var conn net.Conn
	var err error
	if cfg.Port == implicitTLSPort {
		dialer := &net.Dialer{Timeout: smtpDialTimeout}
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12})
	} else {
		conn, err = net.DialTimeout("tcp", addr, smtpDialTimeout)
	}
	if err != nil {
		return fmt.Errorf("connecting to %s: %w", addr, err)
	}
	// The deadline covers the whole conversation, not just the dial: a server that
	// accepts the connection and then stops talking is the case that hangs a worker.
	if err := conn.SetDeadline(time.Now().Add(smtpTotalTimeout)); err != nil {
		_ = conn.Close()
		return err
	}

	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp handshake with %s: %w", cfg.Host, err)
	}
	defer func() { _ = client.Close() }()

	if cfg.Port != implicitTLSPort {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
				return fmt.Errorf("starttls with %s: %w", cfg.Host, err)
			}
		} else if cfg.Username != "" {
			return fmt.Errorf("%s does not offer STARTTLS and this channel has credentials; refusing to send the password in the clear", cfg.Host)
		}
	}

	if cfg.Username != "" {
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("authenticating as %s: %w", cfg.Username, err)
		}
	}

	sender, err := senderAddress(cfg.From)
	if err != nil {
		return err
	}
	if err := client.Mail(sender); err != nil {
		return fmt.Errorf("MAIL FROM %s: %w", sender, err)
	}
	for _, rcpt := range cfg.To {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("RCPT TO %s: %w", rcpt, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	return client.Quit()
}

// senderAddress strips any display name: the envelope wants the bare address even when
// the From header carries "Warden <alerts@example.com>".
func senderAddress(from string) (string, error) {
	parsed, err := mail.ParseAddress(from)
	if err != nil {
		return "", fmt.Errorf("from address %q is not a valid email address", from)
	}
	return parsed.Address, nil
}

// buildMessage assembles a multipart/alternative message: plain text for terminals and
// mail clients that prefer it, HTML for everyone else.
//
// Both parts are base64-encoded rather than sent raw. A monitor name or an error message
// can carry UTF-8 or a line longer than the 998 characters SMTP allows, and either one
// corrupts a raw 8-bit body.
func buildMessage(cfg *emailConfig, subject, textBody, htmlBody string) ([]byte, error) {
	// A newline in a header value would let a monitor name inject headers of its own —
	// an extra Bcc, or a second body. Reject rather than sanitize: none of these fields
	// has any business containing a newline.
	for _, field := range []string{subject, cfg.From, strings.Join(cfg.To, ",")} {
		if strings.ContainsAny(field, "\r\n") {
			return nil, fmt.Errorf("header value contains a line break")
		}
	}

	boundary := "warden-" + strconv.FormatInt(time.Now().UnixNano(), 36)

	var b strings.Builder
	b.WriteString("From: " + cfg.From + "\r\n")
	b.WriteString("To: " + strings.Join(cfg.To, ", ") + "\r\n")
	b.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n")
	b.WriteString("\r\n")

	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	b.WriteString(base64Body(textBody))

	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	b.WriteString(base64Body(htmlBody))

	b.WriteString("--" + boundary + "--\r\n")

	return []byte(b.String()), nil
}

// base64Body encodes a part and wraps it at 76 characters, as RFC 2045 requires.
func base64Body(body string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(body))
	var b strings.Builder
	for len(encoded) > 76 {
		b.WriteString(encoded[:76] + "\r\n")
		encoded = encoded[76:]
	}
	b.WriteString(encoded + "\r\n\r\n")
	return b.String()
}

// eventBody is one alert, ready to render in either format.
type eventBody struct {
	Title   string
	Color   string
	Monitor string
	URL     string
	Message string
	Time    time.Time
}

func (e eventBody) text() string {
	lines := []string{
		e.Title + ": " + e.Monitor,
		"",
		e.Message,
		"",
		"Monitor: " + e.Monitor,
	}
	if e.URL != "" {
		lines = append(lines, "URL:     "+e.URL)
	}
	lines = append(lines, "Time:    "+e.Time.Format(time.RFC1123), "", "— Warden")
	return strings.Join(lines, "\n")
}

func (e eventBody) html() string {
	var rows strings.Builder
	rows.WriteString(htmlRow("Monitor", html.EscapeString(e.Monitor)))
	if e.URL != "" {
		rows.WriteString(htmlRow("URL", html.EscapeString(e.URL)))
	}
	rows.WriteString(htmlRow("Time", html.EscapeString(e.Time.Format(time.RFC1123))))

	return htmlShell(e.Color, html.EscapeString(e.Title),
		`<p style="margin:0 0 20px;font-size:16px;line-height:1.5;color:#0f172a">`+html.EscapeString(e.Message)+`</p>`+
			`<table cellpadding="0" cellspacing="0" style="width:100%;font-size:14px">`+rows.String()+`</table>`)
}

func htmlRow(label, value string) string {
	return `<tr>` +
		`<td style="padding:4px 12px 4px 0;color:#64748b;white-space:nowrap;vertical-align:top">` + label + `</td>` +
		`<td style="padding:4px 0;color:#0f172a;word-break:break-all">` + value + `</td>` +
		`</tr>`
}

// htmlShell wraps content in the table-based layout that mail clients actually render.
// Styles are inline because Gmail strips <style> blocks.
func htmlShell(accent, heading, content string) string {
	return `<!doctype html><html><body style="margin:0;padding:24px;background:#f1f5f9;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">` +
		`<table cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;background:#ffffff;border-radius:8px;overflow:hidden;border:1px solid #e2e8f0">` +
		`<tr><td style="height:4px;background:` + accent + `"></td></tr>` +
		`<tr><td style="padding:24px">` +
		`<h1 style="margin:0 0 16px;font-size:18px;color:#0f172a">` + heading + `</h1>` +
		content +
		`</td></tr>` +
		`<tr><td style="padding:16px 24px;border-top:1px solid #e2e8f0;font-size:12px;color:#94a3b8">Sent by Warden</td></tr>` +
		`</table></body></html>`
}

func digestText(summary digestSummary) string {
	dateStr := summary.Date.Format("January 2, 2006")
	if summary.TotalEvents == 0 {
		return "Daily Monitoring Summary — " + dateStr + "\n\nAll systems operational. No incidents today.\n\n— Warden"
	}

	lines := []string{
		"Daily Monitoring Summary — " + dateStr,
		"",
		fmt.Sprintf("%d events across %d monitors", summary.TotalEvents, summary.MonitorCount),
		"",
	}
	for _, m := range summary.Monitors {
		lines = append(lines, "- "+m.Name+": "+digestMonitorSummary(m))
	}
	if link := buildDigestURL(summary.AppURL, summary.Date); link != "" {
		lines = append(lines, "", "View full report: "+link)
	}
	lines = append(lines, "", "— Warden")
	return strings.Join(lines, "\n")
}

func digestHTML(summary digestSummary) string {
	dateStr := html.EscapeString(summary.Date.Format("January 2, 2006"))

	if summary.TotalEvents == 0 {
		return htmlShell("#22c55e", "Daily Monitoring Summary",
			`<p style="margin:0;font-size:16px;color:#0f172a">All systems operational on `+dateStr+`. No incidents today.</p>`)
	}

	var rows strings.Builder
	for _, m := range summary.Monitors {
		rows.WriteString(htmlRow(html.EscapeString(m.Name), html.EscapeString(digestMonitorSummary(m))))
	}

	content := `<p style="margin:0 0 20px;font-size:16px;color:#0f172a">` +
		fmt.Sprintf("%d events across %d monitors on %s.", summary.TotalEvents, summary.MonitorCount, dateStr) +
		`</p><table cellpadding="0" cellspacing="0" style="width:100%;font-size:14px">` + rows.String() + `</table>`

	if link := buildDigestURL(summary.AppURL, summary.Date); link != "" {
		content += `<p style="margin:20px 0 0;font-size:14px"><a href="` + html.EscapeString(link) + `" style="color:#2563eb">View full report</a></p>`
	}

	return htmlShell("#f59e0b", "Daily Monitoring Summary", content)
}

// digestMonitorSummary renders one monitor's line: "down (3x), ssl_expiring", with the SSL
// message replacing the bare count when there is one, matching the Slack digest.
func digestMonitorSummary(m digestMonitor) string {
	var parts []string
	for _, ec := range m.Events {
		if ec.Type == "ssl_expiring" && m.SSLMessage != "" {
			parts = append(parts, m.SSLMessage)
			continue
		}
		parts = append(parts, fmt.Sprintf("%s (%dx)", eventLabel(ec.Type), ec.Count))
	}
	return strings.Join(parts, ", ")
}
