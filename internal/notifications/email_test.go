package notifications

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/projecthelena/warden/internal/db"
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

// fakeSMTPOptions describes how the server should misbehave. The zero value is a server
// that accepts everything, which is what most tests want; the fields exist so the error
// paths in deliver() can be reached without a real mail server refusing a real message.
type fakeSMTPOptions struct {
	extensions []string // announced in the EHLO reply
	rejectFrom bool     // answer MAIL FROM with a permanent failure
	rejectRcpt string   // answer RCPT TO with a permanent failure for this address
	silent     bool     // accept the connection and then never say anything
}

func newFakeSMTP(t *testing.T, greetingExtensions []string) *fakeSMTP {
	return newFakeSMTPWith(t, fakeSMTPOptions{extensions: greetingExtensions})
}

func newFakeSMTPWith(t *testing.T, opts fakeSMTPOptions) *fakeSMTP {
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
		if opts.silent {
			// Hold the connection open without a greeting. The client is left waiting on
			// a read, which is the case the total deadline exists for.
			_, _ = io.Copy(io.Discard, conn)
			return
		}
		s.serve(conn, opts)
	}()
	return s
}

func (s *fakeSMTP) serve(conn net.Conn, opts fakeSMTPOptions) {
	extensions := opts.extensions
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
			if opts.rejectFrom {
				write("550 sender rejected")
				continue
			}
			s.mu.Lock()
			s.from = extractAngleAddr(line)
			s.mu.Unlock()
			write("250 OK")
		case strings.HasPrefix(cmd, "RCPT TO:"):
			addr := extractAngleAddr(line)
			if opts.rejectRcpt != "" && strings.EqualFold(addr, opts.rejectRcpt) {
				write("550 no such user here")
				continue
			}
			s.mu.Lock()
			s.to = append(s.to, addr)
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

// waitForDelivery polls the fake server, since dispatch happens on the service's worker
// goroutine rather than inline.
func waitForDelivery(t *testing.T, server *fakeSMTP, within time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if _, _, data := server.received(); data != "" {
			return data
		}
		time.Sleep(20 * time.Millisecond)
	}
	return ""
}

// The switch in dispatch is what connects this whole file to the rest of Warden. The
// existing slack dispatch test cannot assert delivery because it would need a real HTTP
// round trip; the fake SMTP server means the email one can.
func TestService_DispatchReachesAnEmailChannel(t *testing.T) {
	server := newFakeSMTP(t, nil)
	host, port := hostPort(t, server.addr)

	store := newTestStore(t)
	if err := store.CreateNotificationChannel(db.NotificationChannel{
		ID:   "nc-email",
		Type: "email",
		Name: "Ops mailbox",
		Config: fmt.Sprintf(`{"host":%q,"port":%q,"from":"warden@example.com","to":"ops@example.com"}`,
			host, port),
		Enabled:   true,
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("CreateNotificationChannel: %v", err)
	}

	svc := NewService(store)
	svc.Start()
	svc.Enqueue(testEvent())

	data := waitForDelivery(t, server, 3*time.Second)
	if data == "" {
		t.Fatal("the alert never reached the email channel")
	}
	if !strings.Contains(data, "Subject: [Warden] Monitor Down: API Gateway") {
		t.Errorf("delivered message has the wrong subject:\n%s", data)
	}
}

// A disabled channel is skipped for every type, and email is the one where "sent anyway"
// would be visible in somebody's inbox rather than a log line.
func TestService_DispatchSkipsADisabledEmailChannel(t *testing.T) {
	server := newFakeSMTP(t, nil)
	host, port := hostPort(t, server.addr)

	store := newTestStore(t)
	if err := store.CreateNotificationChannel(db.NotificationChannel{
		ID:   "nc-off",
		Type: "email",
		Name: "Disabled",
		Config: fmt.Sprintf(`{"host":%q,"port":%q,"from":"warden@example.com","to":"ops@example.com"}`,
			host, port),
		Enabled:   false,
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("CreateNotificationChannel: %v", err)
	}

	svc := NewService(store)
	svc.Start()
	svc.Enqueue(testEvent())

	if data := waitForDelivery(t, server, time.Second); data != "" {
		t.Errorf("a disabled channel received mail:\n%s", data)
	}
}

// SendDigest has its own switch, so email being wired into dispatch says nothing about
// whether the daily summary reaches it.
func TestService_SendDigestReachesAnEmailChannel(t *testing.T) {
	server := newFakeSMTP(t, nil)
	host, port := hostPort(t, server.addr)

	store := newTestStore(t)
	if err := store.CreateNotificationChannel(db.NotificationChannel{
		ID:   "nc-digest",
		Type: "email",
		Name: "Ops mailbox",
		Config: fmt.Sprintf(`{"host":%q,"port":%q,"from":"warden@example.com","to":"ops@example.com"}`,
			host, port),
		Enabled:   true,
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("CreateNotificationChannel: %v", err)
	}

	NewService(store).SendDigest([]db.DigestEvent{
		{MonitorID: "m1", MonitorName: "API Gateway", EventType: "down", Message: "Monitor is down"},
	})

	data := waitForDelivery(t, server, 3*time.Second)
	if data == "" {
		t.Fatal("the digest never reached the email channel")
	}
	// The digest subject carries an em dash, so it goes out RFC 2047 encoded and the raw
	// header does not contain the words. Decode before asserting on it.
	subject := ""
	for _, line := range strings.Split(data, "\r\n") {
		if strings.HasPrefix(line, "Subject: ") {
			decoded, err := new(mime.WordDecoder).DecodeHeader(strings.TrimPrefix(line, "Subject: "))
			if err != nil {
				t.Fatalf("decoding the subject %q: %v", line, err)
			}
			subject = decoded
			break
		}
	}
	if !strings.HasPrefix(subject, "[Warden] Daily Summary") {
		t.Errorf("delivered digest has the wrong subject: %q", subject)
	}
}

// The recipient cap is the difference between a distribution list and an accident. A
// pasted address book would otherwise become one SMTP conversation with hundreds of RCPT
// TO commands, which is how a channel gets a server to rate-limit the whole instance.
func TestEmailConfig_RecipientCap(t *testing.T) {
	addresses := func(n int) string {
		list := make([]string, n)
		for i := range list {
			list[i] = fmt.Sprintf("ops%d@example.com", i)
		}
		return strings.Join(list, ",")
	}

	cfg, err := NewEmailNotifier(fmt.Sprintf(`{"host":"h","from":"a@example.com","to":%q}`, addresses(50))).parseConfig()
	if err != nil {
		t.Fatalf("50 recipients should be accepted: %v", err)
	}
	if len(cfg.To) != 50 {
		t.Errorf("expected 50 recipients, got %d", len(cfg.To))
	}

	err = ValidateEmailConfig(fmt.Sprintf(`{"host":"h","from":"a@example.com","to":%q}`, addresses(51)))
	if err == nil {
		t.Fatal("expected 51 recipients to be rejected")
	}
	if !strings.Contains(err.Error(), "max 50") {
		t.Errorf("expected the error to name the cap, got %v", err)
	}
}

// SMTP allows 998 characters per line. A monitor whose name or error message runs long
// would produce a base64 blob well past that, and a server is entitled to reject or
// truncate it — so every encoded line has to be wrapped.
func TestEmailMessage_WrapsBase64At76Columns(t *testing.T) {
	cfg := &emailConfig{Host: "h", Port: 587, From: "a@example.com", To: []string{"b@example.com"}}
	long := strings.Repeat("the check timed out and the upstream never answered. ", 40)

	msg, err := buildMessage(cfg, "subject", long, "<p>"+long+"</p>")
	if err != nil {
		t.Fatalf("buildMessage: %v", err)
	}

	for i, line := range strings.Split(string(msg), "\r\n") {
		if len(line) > 76 {
			t.Fatalf("line %d is %d characters, past the 76 RFC 2045 allows: %q", i, len(line), line)
		}
	}

	// Wrapped, but still the same bytes once the wrapping is undone.
	var encoded strings.Builder
	for _, line := range strings.Split(string(msg), "\r\n") {
		if line != "" && !strings.Contains(line, " ") && !strings.HasPrefix(line, "--") {
			encoded.WriteString(line)
		}
	}
	if !strings.Contains(encoded.String(), base64.StdEncoding.EncodeToString([]byte(long))) {
		t.Error("the wrapped plain part does not decode back to the body that went in")
	}
}

// Port 465 is TLS from the first byte. Getting this wrong is not a failed send but a
// silent one: the client would sit waiting for a plaintext 220 greeting that a TLS server
// never sends, and the alert would die on the dial timeout.
//
// The port is a variable precisely so this can be checked — 465 is privileged and a test
// binary cannot listen on it. The server here does not speak TLS, so the handshake fails;
// what is being asserted is what Warden put on the wire first.
func TestEmailNotifier_ImplicitTLSOnPort465(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	first := make(chan []byte, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			first <- nil
			return
		}
		defer func() { _ = conn.Close() }()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		buf := make([]byte, 3)
		n, _ := io.ReadFull(conn, buf)
		first <- buf[:n]
	}()

	host, port := hostPort(t, ln.Addr().String())
	portNum, err := strconv.Atoi(port)
	if err != nil {
		t.Fatalf("port %q: %v", port, err)
	}

	original := implicitTLSPort
	implicitTLSPort = portNum
	t.Cleanup(func() { implicitTLSPort = original })

	config := fmt.Sprintf(`{"host":%q,"port":%q,"from":"warden@example.com","to":"ops@example.com"}`, host, port)
	if err := NewEmailNotifier(config).Send(testEvent()); err == nil {
		t.Error("expected the send to fail against a server that does not speak TLS")
	}

	opening := <-first
	if len(opening) < 3 {
		t.Fatalf("the client sent %d bytes before reading; on this port it must open with TLS", len(opening))
	}
	// A TLS record starts with the handshake content type and the protocol version.
	if opening[0] != 0x16 || opening[1] != 0x03 {
		t.Errorf("expected a TLS ClientHello, got % x — the client waited for a plaintext greeting", opening)
	}
}

// A server that takes the message and then refuses a recipient must produce an error that
// names the address. "Send failed" on a channel with six recipients tells the operator
// nothing about which one is wrong.
func TestEmailNotifier_SurfacesARejectedRecipient(t *testing.T) {
	server := newFakeSMTPWith(t, fakeSMTPOptions{rejectRcpt: "gone@example.com"})
	host, port := hostPort(t, server.addr)

	config := fmt.Sprintf(`{"host":%q,"port":%q,"from":"warden@example.com","to":"ops@example.com, gone@example.com"}`, host, port)
	err := NewEmailNotifier(config).Send(testEvent())
	if err == nil {
		t.Fatal("expected an error when the server refuses a recipient")
	}
	if !strings.Contains(err.Error(), "gone@example.com") {
		t.Errorf("expected the error to name the rejected recipient, got %v", err)
	}

	if _, _, data := server.received(); data != "" {
		t.Error("the message was delivered even though a recipient was refused")
	}
}

func TestEmailNotifier_SurfacesARejectedSender(t *testing.T) {
	server := newFakeSMTPWith(t, fakeSMTPOptions{rejectFrom: true})
	host, port := hostPort(t, server.addr)

	config := fmt.Sprintf(`{"host":%q,"port":%q,"from":"warden@example.com","to":"ops@example.com"}`, host, port)
	err := NewEmailNotifier(config).Send(testEvent())
	if err == nil {
		t.Fatal("expected an error when the server refuses the sender")
	}
	if !strings.Contains(err.Error(), "MAIL FROM") {
		t.Errorf("expected the error to name the failed command, got %v", err)
	}
}

// The notification worker drains the queue one event at a time, so a server that accepts
// the connection and then says nothing would hold up every other alert behind it. The
// deadline covers the conversation, not just the dial.
func TestEmailNotifier_GivesUpOnASilentServer(t *testing.T) {
	server := newFakeSMTPWith(t, fakeSMTPOptions{silent: true})
	host, port := hostPort(t, server.addr)

	original := smtpTotalTimeout
	smtpTotalTimeout = 200 * time.Millisecond
	t.Cleanup(func() { smtpTotalTimeout = original })

	config := fmt.Sprintf(`{"host":%q,"port":%q,"from":"warden@example.com","to":"ops@example.com"}`, host, port)

	start := time.Now()
	err := NewEmailNotifier(config).Send(testEvent())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected an error from a server that never answers")
	}
	if elapsed > 5*time.Second {
		t.Errorf("the send took %s; the deadline should have ended it in about %s", elapsed, smtpTotalTimeout)
	}
}

// The digest is a summary, so the link back to the full report is the part that makes it
// actionable. It is only added when there is an app URL to build it from.
func TestEmailDigest_LinksToTheFullReport(t *testing.T) {
	summary := digestSummary{
		TotalEvents:  2,
		MonitorCount: 1,
		Date:         time.Date(2026, 3, 14, 8, 0, 0, 0, time.UTC),
		AppURL:       "https://warden.example.com",
		Monitors: []digestMonitor{{
			Name:   "API Gateway",
			Events: []digestEventCount{{Type: "down", Count: 2}},
		}},
	}

	link := "https://warden.example.com/incidents?date=2026-03-14"
	if !strings.Contains(digestText(summary), link) {
		t.Errorf("the plain text digest should link to the report:\n%s", digestText(summary))
	}
	if !strings.Contains(digestHTML(summary), `href="`+link+`"`) {
		t.Error("the HTML digest should link to the report")
	}

	// Without an app URL there is nothing to link to, and a bare "View full report" with
	// no destination is worse than no line at all.
	summary.AppURL = ""
	if strings.Contains(digestText(summary), "View full report") {
		t.Error("expected no report line when there is no app URL")
	}
}
