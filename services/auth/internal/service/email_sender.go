package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/poro/auth/internal/config"
)

// smtpTimeout bounds one whole SMTP conversation.
const smtpTimeout = 15 * time.Second

// ErrEmailUnavailable is returned when an email cannot be handed to the relay.
var ErrEmailUnavailable = errors.New("email delivery unavailable")

// EmailMessage is a plain-text email to one recipient.
type EmailMessage struct {
	To      string
	Subject string
	Body    string
}

// EmailSender hands an email to a delivery service.
type EmailSender interface {
	Send(ctx context.Context, msg EmailMessage) error
}

// NewEmailSender returns the sender named by EMAIL_PROVIDER.
func NewEmailSender(cfg *config.Config, log *zap.Logger) (EmailSender, error) {
	switch cfg.EmailProvider {
	case config.EmailProviderLog:
		if !cfg.IsDev() {
			return nil, errors.New("email provider log is only allowed in dev")
		}
		return &logEmailSender{log: log}, nil
	case config.EmailProviderSMTP:
		from, err := mail.ParseAddress(cfg.EmailFrom)
		if err != nil {
			return nil, fmt.Errorf("email provider smtp: EMAIL_FROM: %w", err)
		}
		if cfg.SMTPHost == "" || cfg.SMTPPort <= 0 {
			return nil, errors.New("email provider smtp: SMTP_HOST and SMTP_PORT are required")
		}
		return &smtpSender{
			host:     cfg.SMTPHost,
			port:     cfg.SMTPPort,
			username: cfg.SMTPUsername,
			password: cfg.SMTPPassword,
			from:     from,
			tls:      &tls.Config{ServerName: cfg.SMTPHost, MinVersion: tls.VersionTLS12},
			timeout:  smtpTimeout,
			// 465 is the submission port with implicit TLS (RFC 8314).
			implicitTLS: cfg.SMTPPort == 465,
		}, nil
	default:
		return nil, fmt.Errorf("email provider %q is not supported", cfg.EmailProvider)
	}
}

type logEmailSender struct {
	log *zap.Logger
}

// Send writes the email, code included, to the log. Dev only.
func (s *logEmailSender) Send(_ context.Context, msg EmailMessage) error {
	s.log.Info("dev email", zap.String("to", msg.To), zap.String("subject", msg.Subject), zap.String("body", msg.Body))
	return nil
}

// smtpSender talks to a relay over TLS only: implicit TLS on port 465,
// mandatory STARTTLS elsewhere. A relay without STARTTLS is refused.
type smtpSender struct {
	host     string
	port     int
	username string
	password string
	from     *mail.Address
	tls      *tls.Config
	timeout  time.Duration

	implicitTLS bool
}

func (s *smtpSender) Send(ctx context.Context, msg EmailMessage) error {
	to, err := mail.ParseAddress(msg.To)
	if err != nil {
		return fmt.Errorf("send email: recipient: %w", err)
	}
	body, err := buildEmail(s.from, to, msg, time.Now())
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	if err := s.deliver(ctx, to.Address, body); err != nil {
		return fmt.Errorf("%w: %w", ErrEmailUnavailable, err)
	}
	return nil
}

func (s *smtpSender) deliver(ctx context.Context, to string, body []byte) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	addr := net.JoinHostPort(s.host, strconv.Itoa(s.port))
	dialer := &net.Dialer{}
	implicitTLS := s.implicitTLS

	var conn net.Conn
	var err error
	if implicitTLS {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: s.tls}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			_ = conn.Close()
			return fmt.Errorf("set deadline: %w", err)
		}
	}
	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp greeting: %w", err)
	}
	defer func() { _ = client.Close() }()

	if !implicitTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("relay does not offer STARTTLS")
		}
		if err := client.StartTLS(s.tls); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}
	if s.username != "" {
		if err := client.Auth(smtp.PlainAuth("", s.username, s.password, s.host)); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}
	if err := client.Mail(s.from.Address); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("rcpt to: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err := w.Write(body); err != nil {
		return fmt.Errorf("write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("end data: %w", err)
	}
	return client.Quit()
}

// buildEmail writes a UTF-8 plain-text message with CRLF line endings. The
// subject is RFC 2047 encoded and the body quoted-printable, so accents
// survive any relay.
func buildEmail(from, to *mail.Address, msg EmailMessage, now time.Time) ([]byte, error) {
	if strings.ContainsAny(msg.Subject, "\r\n") {
		return nil, errors.New("subject must be one line")
	}
	domain := from.Address[strings.LastIndex(from.Address, "@")+1:]
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("message id: %w", err)
	}
	var buf bytes.Buffer
	for _, h := range [][2]string{
		{"From", from.String()},
		{"To", to.String()},
		{"Subject", mime.QEncoding.Encode("utf-8", msg.Subject)},
		{"Date", now.Format(time.RFC1123Z)},
		{"Message-ID", "<" + id.String() + "@" + domain + ">"},
		{"MIME-Version", "1.0"},
		{"Content-Type", "text/plain; charset=UTF-8"},
		{"Content-Transfer-Encoding", "quoted-printable"},
	} {
		buf.WriteString(h[0] + ": " + h[1] + "\r\n")
	}
	buf.WriteString("\r\n")
	qp := quotedprintable.NewWriter(&buf)
	if _, err := qp.Write([]byte(strings.ReplaceAll(msg.Body, "\n", "\r\n"))); err != nil {
		return nil, fmt.Errorf("encode body: %w", err)
	}
	if err := qp.Close(); err != nil {
		return nil, fmt.Errorf("encode body: %w", err)
	}
	return buf.Bytes(), nil
}
