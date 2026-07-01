package smtp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"anchor/internal/integration/provider"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

// Provider-agnostic SMTP transport. Speaks SMTP-AUTH (PLAIN or LOGIN) over
// implicit TLS (port 465) or STARTTLS (port 587). Validated against
// SendGrid, Mailgun, Proton, and AWS SES — the four hosted providers we
// committed to supporting in v1. Self-hosted relays that follow RFC 5321
// will work too.

const (
	asciiMaxRune = 127
	dialTimeout  = 30 * time.Second
)

// dialSMTP returns a connected smtp.Client for the given config, with TLS
// established as required.
func dialSMTP(ctx context.Context, cfg Config) (*smtp.Client, error) {
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		deadline = time.Now().Add(dialTimeout)
	}
	dialer := &net.Dialer{Deadline: deadline}

	switch cfg.Encryption {
	case EncryptionTLS:
		tlsCfg := &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}
		tlsDialer := &tls.Dialer{NetDialer: dialer, Config: tlsCfg}
		conn, err := tlsDialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("smtp tls dial: %w", err)
		}
		if deadlineErr := conn.SetDeadline(deadline); deadlineErr != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("smtp set deadline: %w", deadlineErr)
		}
		client, err := smtp.NewClient(conn, cfg.Host)
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("smtp client init: %w", err)
		}
		return client, nil

	case EncryptionStartTLS, "":
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("smtp dial: %w", err)
		}
		if deadlineErr := conn.SetDeadline(deadline); deadlineErr != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("smtp set deadline: %w", deadlineErr)
		}
		client, err := smtp.NewClient(conn, cfg.Host)
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("smtp client init: %w", err)
		}
		if ok, _ := client.Extension("STARTTLS"); !ok {
			_ = client.Close()
			return nil, errors.New("smtp server does not advertise STARTTLS")
		}
		tlsCfg := &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}
		if startTLSErr := client.StartTLS(tlsCfg); startTLSErr != nil {
			_ = client.Close()
			return nil, fmt.Errorf("smtp starttls: %w", startTLSErr)
		}
		return client, nil

	case EncryptionNone:
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("smtp dial: %w", err)
		}
		if deadlineErr := conn.SetDeadline(deadline); deadlineErr != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("smtp set deadline: %w", deadlineErr)
		}
		client, err := smtp.NewClient(conn, cfg.Host)
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("smtp client init: %w", err)
		}
		return client, nil

	default:
		return nil, fmt.Errorf("unknown smtp encryption %q", cfg.Encryption)
	}
}

// authenticate negotiates SASL using the configured mechanism. PLAIN is the
// default and is what every hosted provider we support advertises.
func authenticate(client *smtp.Client, cfg Config) error {
	if strings.TrimSpace(cfg.Username) == "" {
		return nil
	}
	if ok, _ := client.Extension("AUTH"); !ok {
		return errors.New("smtp server does not advertise AUTH")
	}
	switch cfg.AuthMethod {
	case AuthMethodLogin:
		return client.Auth(loginAuth(cfg.Username, cfg.Password, cfg.Host))
	case AuthMethodPlain, "":
		return client.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host))
	default:
		return fmt.Errorf("unknown auth method %q", cfg.AuthMethod)
	}
}

// loginAuth implements the LOGIN mechanism. Some legacy relays advertise
// LOGIN but not PLAIN; net/smtp ships PLAIN/CRAM-MD5 only, so we provide
// our own minimal LOGIN client.
type loginAuthClient struct {
	username, password, host string
}

func loginAuth(username, password, host string) smtp.Auth {
	return &loginAuthClient{username: username, password: password, host: host}
}

func (a *loginAuthClient) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if !server.TLS && a.host != "localhost" && a.host != "127.0.0.1" {
		return "", nil, errors.New("smtp login auth requires TLS")
	}
	if server.Name != a.host {
		return "", nil, fmt.Errorf("smtp login auth: server name %q does not match config host %q", server.Name, a.host)
	}
	return "LOGIN", nil, nil
}

func (a *loginAuthClient) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	switch strings.TrimSpace(strings.ToLower(string(fromServer))) {
	case "username:":
		return []byte(a.username), nil
	case "password:":
		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("unexpected smtp login challenge %q", string(fromServer))
	}
}

// sendMessage formats msg as a multipart/alternative RFC 5322 message and
// hands it off to the SMTP server already authenticated on `client`.
func sendMessage(client *smtp.Client, cfg Config, msg provider.OutboundMessage) (string, error) {
	from := msg.From
	if strings.TrimSpace(from.Email) == "" {
		from = provider.Address{Email: cfg.FromAddress, Name: firstNonEmpty(msg.From.Name, cfg.FromName)}
	}
	if strings.TrimSpace(from.Email) == "" {
		return "", errors.New("smtp send: from address is empty")
	}
	if len(msg.To) == 0 {
		return "", errors.New("smtp send: at least one recipient is required")
	}

	if err := client.Mail(from.Email); err != nil {
		return "", classifyEnvelopeErr(fmt.Errorf("smtp MAIL FROM: %w", err), err)
	}
	for _, to := range msg.To {
		if strings.TrimSpace(to.Email) == "" {
			return "", errors.New("smtp send: recipient address is empty")
		}
		if err := client.Rcpt(to.Email); err != nil {
			return "", classifyEnvelopeErr(fmt.Errorf("smtp RCPT TO %s: %w", to.Email, err), err)
		}
	}

	wc, err := client.Data()
	if err != nil {
		return "", fmt.Errorf("smtp DATA: %w", err)
	}

	messageID := msg.MessageID
	if strings.TrimSpace(messageID) == "" {
		messageID = generateMessageID(from.Email)
	}

	body, err := buildRFC5322Message(from, msg.To, msg.ReplyTo, messageID, msg.Subject, msg.HTML, msg.Text, msg.Headers)
	if err != nil {
		_ = wc.Close()
		return "", err
	}
	if _, writeErr := wc.Write(body); writeErr != nil {
		_ = wc.Close()
		return "", fmt.Errorf("smtp DATA write: %w", writeErr)
	}
	if closeErr := wc.Close(); closeErr != nil {
		return "", fmt.Errorf("smtp DATA close: %w", closeErr)
	}
	return messageID, nil
}

// classifyEnvelopeErr tags a MAIL FROM / RCPT TO failure as a permanent message
// rejection when the relay replied with a 5xx SMTP code. wrapped is the
// log-friendly error (it keeps the command and address context); cause is the
// raw error returned by net/smtp, inspected for the protocol reply code. A
// permanent rejection is the caller's problem (unauthorized sender, unknown
// recipient), so it is wrapped with provider.ErrMessageRejected; transient
// failures and non-protocol errors propagate unchanged as ordinary delivery
// failures the caller may retry.
func classifyEnvelopeErr(wrapped, cause error) error {
	if isPermanentSMTPRejection(cause) {
		return fmt.Errorf("%w: %w", provider.ErrMessageRejected, wrapped)
	}
	return wrapped
}

// isPermanentSMTPRejection reports whether err carries an SMTP protocol reply
// with a permanent (5xx) negative-completion code. Per RFC 5321 §4.2.1 a 5yz
// reply means the server will never accept the command as issued, whereas a 4yz
// reply is transient and worth retrying.
func isPermanentSMTPRejection(err error) bool {
	var protoErr *textproto.Error
	if !errors.As(err, &protoErr) {
		return false
	}
	return protoErr.Code >= 500 && protoErr.Code < 600
}

// buildRFC5322Message renders an RFC 5322 multipart/alternative message. We
// always include both text and HTML parts: the text part is auto-derived
// when not supplied to keep deliverability sane (a missing text part trips
// some spam filters).
func buildRFC5322Message(
	from provider.Address,
	to []provider.Address,
	replyTo *provider.Address,
	messageID string,
	subject, html, text string,
	extraHeaders map[string]string,
) ([]byte, error) {
	var b strings.Builder

	hdr := textproto.MIMEHeader{}
	hdr.Set("From", formatAddress(from))
	hdr.Set("To", formatAddressList(to))
	if replyTo != nil && strings.TrimSpace(replyTo.Email) != "" {
		hdr.Set("Reply-To", formatAddress(*replyTo))
	}
	hdr.Set("Subject", encodeHeader(subject))
	hdr.Set("Message-ID", "<"+messageID+">")
	hdr.Set("Date", time.Now().UTC().Format(time.RFC1123Z))
	hdr.Set("MIME-Version", "1.0")
	for k, v := range extraHeaders {
		hdr.Set(k, v)
	}

	boundary := "anchor-bnd-" + ids.MustNew("mp")
	hdr.Set("Content-Type", `multipart/alternative; boundary="`+boundary+`"`)

	for k, vs := range hdr {
		for _, v := range vs {
			b.WriteString(k)
			b.WriteString(": ")
			b.WriteString(v)
			b.WriteString("\r\n")
		}
	}
	b.WriteString("\r\n")

	textPart := text
	if strings.TrimSpace(textPart) == "" {
		textPart = stripHTMLToText(html)
	}

	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	if err := writeQuotedPrintable(&b, textPart); err != nil {
		return nil, err
	}
	b.WriteString("\r\n")

	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	if err := writeQuotedPrintable(&b, html); err != nil {
		return nil, err
	}
	b.WriteString("\r\n")

	b.WriteString("--" + boundary + "--\r\n")
	return []byte(b.String()), nil
}

func writeQuotedPrintable(w *strings.Builder, s string) error {
	qp := quotedprintable.NewWriter(stringWriter{w})
	if _, err := qp.Write([]byte(s)); err != nil {
		return err
	}
	return qp.Close()
}

// stringWriter adapts strings.Builder to io.Writer.
type stringWriter struct{ b *strings.Builder }

func (s stringWriter) Write(p []byte) (int, error) { return s.b.Write(p) }

func formatAddress(a provider.Address) string {
	addr := mail.Address{Name: a.Name, Address: a.Email}
	return addr.String()
}

func formatAddressList(addrs []provider.Address) string {
	parts := make([]string, 0, len(addrs))
	for _, a := range addrs {
		parts = append(parts, formatAddress(a))
	}
	return strings.Join(parts, ", ")
}

// encodeHeader encodes non-ASCII header values via RFC 2047 Q-encoding.
// net/mail.Address.String already handles names; subject does not.
func encodeHeader(s string) string {
	for _, r := range s {
		if r > asciiMaxRune {
			return mimeBEncodingHeader(s)
		}
	}
	return s
}

// mimeBEncodingHeader is a thin wrapper that escapes the import cycle
// (mime.BEncoding lives in mime, but using it here keeps the import
// surface lean).
func mimeBEncodingHeader(s string) string {
	return mimeBEncode(s)
}

// generateMessageID returns an RFC 5322 Message-ID derived from a KSUID and
// the sender domain. Hosted providers will overwrite this in their relay,
// but having one on submission helps tracing on self-hosted relays.
func generateMessageID(fromEmail string) string {
	domain := "anchor.local"
	if at := strings.LastIndex(fromEmail, "@"); at >= 0 && at < len(fromEmail)-1 {
		domain = fromEmail[at+1:]
	}
	return ids.MustNew("emid") + "@" + domain
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// stripHTMLToText is a deliberately tiny HTML→text fallback for the case
// where the caller did not supply a plain-text body. We do not pull in a
// full sanitizer here — customers who care about pixel-perfect text
// alternates should provide their own text body.
func stripHTMLToText(html string) string {
	var b strings.Builder
	inTag := false
	for _, r := range html {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
			b.WriteRune(' ')
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	return strings.TrimSpace(b.String())
}
