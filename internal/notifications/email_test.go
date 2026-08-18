package notifications

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"mime"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSMTP is a single-connection SMTP server that speaks just enough of the protocol to
// let a client deliver one message. It records what it was told so the tests can assert
// on the envelope, not only on the absence of an error.
type fakeSMTP struct {
	addr string

	mu       sync.Mutex
	from     string
	to       []string
	data     string
	startTLS bool
}

func newFakeSMTP(t *testing.T, greetingExtensions []string) *fakeSMTP {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	s := &fakeSMTP{addr: ln.Addr().String()}
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		s.serve(conn, greetingExtensions)
	}()
	return s
}

func (s *fakeSMTP) serve(conn net.Conn, extensions []string) {
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	r := bufio.NewReader(conn)
	write := func(line string) { _, _ = fmt.Fprintf(conn, "%s\r\n", line) }

	write("220 fake ESMTP")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))

		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			// The first line of an EHLO reply is the greeting, not an extension —
			// net/smtp drops it before parsing the rest.
			write("250-fake greets you")
			for _, ext := range extensions {
				write("250-" + ext)
			}
			write("250 SIZE 10240000")
		case strings.HasPrefix(cmd, "MAIL FROM:"):
			s.mu.Lock()
			s.from = extractAngleAddr(line)
			s.mu.Unlock()
			write("250 OK")
		case strings.HasPrefix(cmd, "RCPT TO:"):
			s.mu.Lock()
			s.to = append(s.to, extractAngleAddr(line))
			s.mu.Unlock()
			write("250 OK")
		case cmd == "DATA":
			write("354 send it")
			var body strings.Builder
			for {
				dataLine, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if strings.TrimRight(dataLine, "\r\n") == "." {
					break
				}
				body.WriteString(dataLine)
			}
			s.mu.Lock()
			s.data = body.String()
			s.mu.Unlock()
			write("250 queued")
		case cmd == "STARTTLS":
			s.mu.Lock()
			s.startTLS = true
			s.mu.Unlock()
			// Answering 220 and then not speaking TLS is enough: the test only needs to
			// know that the client asked, and the failed handshake proves it did not
			// carry on in the clear.
			write("220 go ahead")
		case cmd == "QUIT":
			write("221 bye")
			return
		default:
			write("250 OK")
		}
	}
}

func (s *fakeSMTP) received() (string, []string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.from, s.to, s.data
}

func (s *fakeSMTP) sawSTARTTLS() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startTLS
}

func extractAngleAddr(line string) string {
	start := strings.Index(line, "<")
	end := strings.Index(line, ">")
	if start == -1 || end == -1 || end < start {
		return ""
	}
	return line[start+1 : end]
}

func hostPort(t *testing.T, addr string) (string, string) {
	t.Helper()
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split %q: %v", addr, err)
	}
	return host, port
}

func testEvent() NotificationEvent {
	return NotificationEvent{
		MonitorID:   "m-1",
		MonitorName: "API Gateway",
		MonitorURL:  "https://api.example.com",
		Type:        EventDown,
		Message:     "Connection refused",
		Time:        time.Date(2026, 3, 14, 15, 9, 26, 0, time.UTC),
	}
}

func TestEmailConfig_Defaults(t *testing.T) {
	cfg, err := NewEmailNotifier(`{"host":"smtp.example.com","from":"warden@example.com","to":"ops@example.com"}`).parseConfig()
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.Port != 587 {
		t.Errorf("expected the submission port 587 by default, got %d", cfg.Port)
	}
	if len(cfg.To) != 1 || cfg.To[0] != "ops@example.com" {
		t.Errorf("unexpected recipients: %v", cfg.To)
	}
}

func TestEmailConfig_NumericPortFromJSON(t *testing.T) {
	cfg, err := NewEmailNotifier(`{"host":"smtp.example.com","port":465,"from":"a@example.com","to":"b@example.com"}`).parseConfig()
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.Port != 465 {
		t.Errorf("expected port 465, got %d", cfg.Port)
	}
}

func TestEmailConfig_MultipleRecipients(t *testing.T) {
	cfg, err := NewEmailNotifier(`{"host":"h","from":"a@example.com","to":" b@example.com , c@example.com ,"}`).parseConfig()
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if len(cfg.To) != 2 {
		t.Fatalf("expected 2 recipients, got %v", cfg.To)
	}
}

func TestEmailConfig_DisplayNameRecipient(t *testing.T) {
	cfg, err := NewEmailNotifier(`{"host":"h","from":"Warden <a@example.com>","to":"Ops Team <b@example.com>"}`).parseConfig()
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.To[0] != "b@example.com" {
		t.Errorf("expected the bare address for the envelope, got %q", cfg.To[0])
	}
}

func TestEmailConfig_Rejections(t *testing.T) {
	cases := map[string]string{
		"missing host":      `{"from":"a@example.com","to":"b@example.com"}`,
		"missing from":      `{"host":"h","to":"b@example.com"}`,
		"invalid from":      `{"host":"h","from":"not-an-address","to":"b@example.com"}`,
		"missing to":        `{"host":"h","from":"a@example.com"}`,
		"invalid recipient": `{"host":"h","from":"a@example.com","to":"b@example.com, nope"}`,
		"invalid port":      `{"host":"h","port":"70000","from":"a@example.com","to":"b@example.com"}`,
	}
	for name, config := range cases {
		if err := ValidateEmailConfig(config); err == nil {
			t.Errorf("%s: expected an error, got none", name)
		}
	}
}

func TestEmailMessage_Structure(t *testing.T) {
	cfg := &emailConfig{Host: "h", Port: 587, From: "Warden <warden@example.com>", To: []string{"ops@example.com", "cto@example.com"}}
	msg, err := buildMessage(cfg, "[Warden] Monitor Down: API Gateway", "plain body", "<p>html body</p>")
	if err != nil {
		t.Fatalf("buildMessage: %v", err)
	}
	out := string(msg)

	for _, want := range []string{
		"From: Warden <warden@example.com>\r\n",
		"To: ops@example.com, cto@example.com\r\n",
		"Subject: [Warden] Monitor Down: API Gateway\r\n",
		"MIME-Version: 1.0\r\n",
		"multipart/alternative",
		"Content-Type: text/plain; charset=\"utf-8\"",
		"Content-Type: text/html; charset=\"utf-8\"",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("message is missing %q", want)
		}
	}

	if !strings.Contains(out, base64.StdEncoding.EncodeToString([]byte("plain body"))) {
		t.Error("plain part is not base64-encoded")
	}
	if !strings.Contains(out, base64.StdEncoding.EncodeToString([]byte("<p>html body</p>"))) {
		t.Error("html part is not base64-encoded")
	}
}

func TestEmailMessage_EncodesNonASCIISubject(t *testing.T) {
	cfg := &emailConfig{Host: "h", Port: 587, From: "a@example.com", To: []string{"b@example.com"}}
	msg, err := buildMessage(cfg, "Monitor caído: Búsqueda", "text", "html")
	if err != nil {
		t.Fatalf("buildMessage: %v", err)
	}
	subject := ""
	for _, line := range strings.Split(string(msg), "\r\n") {
		if strings.HasPrefix(line, "Subject: ") {
			subject = line
			break
		}
	}
	if strings.Contains(subject, "caído") {
		t.Errorf("non-ASCII subject went out raw: %q", subject)
	}
	if !strings.Contains(subject, "=?utf-8?") {
		t.Errorf("expected an RFC 2047 encoded subject, got %q", subject)
	}
}

// A monitor name is user input and ends up in the Subject header. A newline there would
// let it append headers of its own.
func TestEmailMessage_RejectsHeaderInjection(t *testing.T) {
	cfg := &emailConfig{Host: "h", Port: 587, From: "a@example.com", To: []string{"b@example.com"}}
	if _, err := buildMessage(cfg, "Down\r\nBcc: attacker@evil.com", "text", "html"); err == nil {
		t.Error("expected a message with a newline in the subject to be rejected")
	}

	cfgBadFrom := &emailConfig{Host: "h", Port: 587, From: "a@example.com\r\nBcc: attacker@evil.com", To: []string{"b@example.com"}}
	if _, err := buildMessage(cfgBadFrom, "subject", "text", "html"); err == nil {
		t.Error("expected a message with a newline in the From header to be rejected")
	}
}

func TestEmailBody_EscapesHTML(t *testing.T) {
	body := eventBody{
		Title:   "Monitor Down",
		Color:   "#dc3545",
		Monitor: `<script>alert(1)</script>`,
		URL:     "https://example.com",
		Message: "5xx & rising",
		Time:    time.Now(),
	}
	rendered := body.html()
	if strings.Contains(rendered, "<script>") {
		t.Error("monitor name was not escaped in the HTML body")
	}
	if !strings.Contains(rendered, "5xx &amp; rising") {
		t.Error("expected the message to be HTML-escaped")
	}

	if !strings.Contains(body.text(), "5xx & rising") {
		t.Error("the plain text part should carry the message unescaped")
	}
}

func TestEmailNotifier_SendsOverSMTP(t *testing.T) {
	server := newFakeSMTP(t, nil)
	host, port := hostPort(t, server.addr)

	config := fmt.Sprintf(`{"host":%q,"port":%q,"from":"Warden <warden@example.com>","to":"ops@example.com"}`, host, port)
	if err := NewEmailNotifier(config).Send(testEvent()); err != nil {
		t.Fatalf("Send: %v", err)
	}

	from, to, data := server.received()
	if from != "warden@example.com" {
		t.Errorf("envelope sender should be the bare address, got %q", from)
	}
	if len(to) != 1 || to[0] != "ops@example.com" {
		t.Errorf("unexpected envelope recipients: %v", to)
	}
	if !strings.Contains(data, "Subject: [Warden] Monitor Down: API Gateway") {
		t.Errorf("unexpected subject in delivered message:\n%s", data)
	}
	if !strings.Contains(data, base64.StdEncoding.EncodeToString([]byte("Connection refused"))[:20]) {
		t.Error("delivered message does not carry the alert message")
	}
}

func TestEmailNotifier_SendDirect(t *testing.T) {
	server := newFakeSMTP(t, nil)
	host, port := hostPort(t, server.addr)

	config := fmt.Sprintf(`{"host":%q,"port":%q,"from":"warden@example.com","to":"ops@example.com"}`, host, port)
	if err := SendDirect("email", config, testEvent()); err != nil {
		t.Fatalf("SendDirect: %v", err)
	}
	if _, to, _ := server.received(); len(to) != 1 {
		t.Errorf("expected the test message to be delivered, got recipients %v", to)
	}
}

// The password must never travel in the clear. When the server offers no STARTTLS and the
// channel has credentials, the send fails instead of authenticating over plaintext.
func TestEmailNotifier_RefusesPlaintextAuth(t *testing.T) {
	server := newFakeSMTP(t, nil)
	host, port := hostPort(t, server.addr)

	config := fmt.Sprintf(`{"host":%q,"port":%q,"username":"u","password":"p","from":"warden@example.com","to":"ops@example.com"}`, host, port)
	err := NewEmailNotifier(config).Send(testEvent())
	if err == nil {
		t.Fatal("expected the send to fail rather than authenticate over plaintext")
	}
	if !strings.Contains(err.Error(), "STARTTLS") {
		t.Errorf("expected the error to name STARTTLS, got %v", err)
	}
}

func TestEmailNotifier_SendsDigest(t *testing.T) {
	server := newFakeSMTP(t, nil)
	host, port := hostPort(t, server.addr)

	summary := digestSummary{
		TotalEvents:  3,
		MonitorCount: 1,
		Date:         time.Date(2026, 3, 14, 8, 0, 0, 0, time.UTC),
		Monitors: []digestMonitor{{
			ID:     "m-1",
			Name:   "API Gateway",
			Events: []digestEventCount{{Type: "down", Count: 3}},
		}},
	}

	config := fmt.Sprintf(`{"host":%q,"port":%q,"from":"warden@example.com","to":"ops@example.com"}`, host, port)
	if err := NewEmailNotifier(config).sendDigest(summary); err != nil {
		t.Fatalf("sendDigest: %v", err)
	}

	_, _, data := server.received()
	subject := decodedSubject(t, data)
	if !strings.HasPrefix(subject, "[Warden] Daily Summary") {
		t.Errorf("unexpected digest subject: %q", subject)
	}
}

// decodedSubject pulls the Subject header out of a delivered message and undoes the
// RFC 2047 encoding, which kicks in as soon as the subject carries a non-ASCII character.
func decodedSubject(t *testing.T, message string) string {
	t.Helper()
	for _, line := range strings.Split(message, "\r\n") {
		if !strings.HasPrefix(line, "Subject: ") {
			continue
		}
		decoded, err := (&mime.WordDecoder{}).DecodeHeader(strings.TrimPrefix(line, "Subject: "))
		if err != nil {
			t.Fatalf("decoding subject %q: %v", line, err)
		}
		return decoded
	}
	t.Fatalf("no Subject header in message:\n%s", message)
	return ""
}

func TestEmailDigest_QuietDay(t *testing.T) {
	summary := digestSummary{Date: time.Date(2026, 3, 14, 8, 0, 0, 0, time.UTC)}

	text := digestText(summary)
	if !strings.Contains(text, "All systems operational") {
		t.Errorf("expected the quiet-day wording, got %q", text)
	}
	if !strings.Contains(digestHTML(summary), "All systems operational") {
		t.Error("expected the quiet-day wording in the HTML digest too")
	}
}

func TestEmailDigest_SSLMessageReplacesCount(t *testing.T) {
	summary := digestSummary{
		TotalEvents:  1,
		MonitorCount: 1,
		Date:         time.Now(),
		Monitors: []digestMonitor{{
			Name:       "API Gateway",
			Events:     []digestEventCount{{Type: "ssl_expiring", Count: 1}},
			SSLMessage: "SSL certificate expires in 14 days",
		}},
	}
	if !strings.Contains(digestText(summary), "expires in 14 days") {
		t.Error("the SSL message should replace the bare count, as it does in the Slack digest")
	}
}

func TestEmailNotifier_UnreachableServer(t *testing.T) {
	// Port 1 on loopback refuses connections immediately.
	config := `{"host":"127.0.0.1","port":"1","from":"warden@example.com","to":"ops@example.com"}`
	if err := NewEmailNotifier(config).Send(testEvent()); err == nil {
		t.Error("expected an error when the server is unreachable")
	}
}

// When the server offers STARTTLS, Warden takes it — even without credentials to protect.
// The fake server announces the extension and then refuses to speak TLS, so the send
// fails; what matters is that it failed at the handshake rather than carrying on in
// plaintext.
func TestEmailNotifier_UsesSTARTTLSWhenOffered(t *testing.T) {
	server := newFakeSMTP(t, []string{"STARTTLS"})
	host, port := hostPort(t, server.addr)

	config := fmt.Sprintf(`{"host":%q,"port":%q,"from":"warden@example.com","to":"ops@example.com"}`, host, port)
	err := NewEmailNotifier(config).Send(testEvent())

	if !server.sawSTARTTLS() {
		t.Error("client did not attempt STARTTLS even though the server offered it")
	}
	if err == nil {
		t.Error("expected the send to fail when the TLS handshake does not complete")
	}
	if _, _, data := server.received(); data != "" {
		t.Error("message was delivered despite the failed TLS handshake")
	}
}
