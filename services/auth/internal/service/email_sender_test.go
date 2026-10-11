package service

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"io"
	"math/big"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/poro/auth/internal/config"
)

func TestNewEmailSender(t *testing.T) {
	dev := &config.Config{AppEnv: config.EnvDev, EmailProvider: config.EmailProviderLog}
	sender, err := NewEmailSender(dev, zap.NewNop())
	require.NoError(t, err)
	require.IsType(t, &logEmailSender{}, sender)

	prodLog := &config.Config{AppEnv: config.EnvProd, EmailProvider: config.EmailProviderLog}
	_, err = NewEmailSender(prodLog, zap.NewNop())
	require.Error(t, err)

	smtpCfg := &config.Config{EmailProvider: config.EmailProviderSMTP, EmailFrom: "PORO <no-reply@poro.test>", SMTPHost: "smtp.poro.test", SMTPPort: 465}
	sender, err = NewEmailSender(smtpCfg, zap.NewNop())
	require.NoError(t, err)
	s := sender.(*smtpSender)
	require.True(t, s.implicitTLS)
	require.Equal(t, "no-reply@poro.test", s.from.Address)
	require.Equal(t, uint16(tls.VersionTLS12), s.tls.MinVersion)

	smtpCfg.SMTPPort = 587
	sender, err = NewEmailSender(smtpCfg, zap.NewNop())
	require.NoError(t, err)
	require.False(t, sender.(*smtpSender).implicitTLS)

	bad := *smtpCfg
	bad.EmailFrom = "not an address"
	_, err = NewEmailSender(&bad, zap.NewNop())
	require.Error(t, err)
	bad = *smtpCfg
	bad.SMTPHost = ""
	_, err = NewEmailSender(&bad, zap.NewNop())
	require.Error(t, err)
	_, err = NewEmailSender(&config.Config{EmailProvider: "sendmail"}, zap.NewNop())
	require.Error(t, err)
}

func TestLogEmailSenderWritesTheCode(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	sender := &logEmailSender{log: zap.New(core)}
	require.NoError(t, sender.Send(context.Background(), EmailMessage{To: "awa@poro.test", Subject: "s", Body: "code 123456"}))
	entry := logs.All()[0]
	require.Equal(t, "dev email", entry.Message)
	require.Equal(t, "code 123456", entry.ContextMap()["body"])
}

func TestBuildEmailEncodesFrenchText(t *testing.T) {
	from := &mail.Address{Name: "PORO", Address: "no-reply@poro.test"}
	to := &mail.Address{Address: "awa@poro.test"}
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	raw, err := buildEmail(from, to, EmailMessage{Subject: "Réinitialise ton mot de passe", Body: "Bonjour,\nVoici ton code : 123456 — à bientôt\n"}, now)
	require.NoError(t, err)

	parsed, err := mail.ReadMessage(strings.NewReader(string(raw)))
	require.NoError(t, err)
	subject, err := new(mime.WordDecoder).DecodeHeader(parsed.Header.Get("Subject"))
	require.NoError(t, err)
	require.Equal(t, "Réinitialise ton mot de passe", subject)
	require.Equal(t, `"PORO" <no-reply@poro.test>`, parsed.Header.Get("From"))
	require.Equal(t, "<awa@poro.test>", parsed.Header.Get("To"))
	require.Equal(t, "text/plain; charset=UTF-8", parsed.Header.Get("Content-Type"))
	require.True(t, strings.HasSuffix(parsed.Header.Get("Message-ID"), "@poro.test>"))
	require.Equal(t, "Mon, 12 Oct 2026 09:00:00 +0000", parsed.Header.Get("Date"))
	body, err := io.ReadAll(quotedprintable.NewReader(parsed.Body))
	require.NoError(t, err)
	require.Equal(t, "Bonjour,\r\nVoici ton code : 123456 — à bientôt\r\n", string(body))
	require.NotContains(t, string(raw), "\n\n", "every line ends with CRLF")

	_, err = buildEmail(from, to, EmailMessage{Subject: "a\r\nBcc: evil@poro.test"}, now)
	require.Error(t, err, "no header injection through the subject")
}

func TestSMTPSenderSTARTTLS(t *testing.T) {
	srv := startFakeSMTP(t, false, true)
	sender := srv.sender(t, false)
	sender.username, sender.password = "apikey", "secret-key"

	err := sender.Send(context.Background(), EmailMessage{To: "awa@poro.test", Subject: "Ton code", Body: "Code : 123456\n"})
	require.NoError(t, err)

	got := srv.received(t)
	require.True(t, got.tls, "the message travelled over TLS")
	require.Equal(t, "\x00apikey\x00secret-key", got.auth)
	require.Equal(t, "<no-reply@poro.test>", got.from)
	require.Equal(t, []string{"<awa@poro.test>"}, got.to)
	require.Contains(t, got.data, "Code : 123456")
}

func TestSMTPSenderImplicitTLS(t *testing.T) {
	srv := startFakeSMTP(t, true, false)
	sender := srv.sender(t, true)
	require.NoError(t, sender.Send(context.Background(), EmailMessage{To: "awa@poro.test", Subject: "Ton code", Body: "123456"}))
	got := srv.received(t)
	require.True(t, got.tls)
	require.Empty(t, got.auth, "no credentials configured, no AUTH")
}

func TestSMTPSenderRefusesPlaintextRelays(t *testing.T) {
	srv := startFakeSMTP(t, false, false)
	err := srv.sender(t, false).Send(context.Background(), EmailMessage{To: "awa@poro.test", Subject: "s", Body: "b"})
	require.ErrorIs(t, err, ErrEmailUnavailable)
	require.ErrorContains(t, err, "STARTTLS")
	require.Empty(t, srv.messages(), "nothing is sent in clear text")
}

func TestSMTPSenderErrors(t *testing.T) {
	srv := startFakeSMTP(t, false, true)
	sender := srv.sender(t, false)

	err := sender.Send(context.Background(), EmailMessage{To: "not an address", Subject: "s", Body: "b"})
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrEmailUnavailable, "a bad address is not an outage")

	untrusted := *sender
	untrusted.tls = &tls.Config{ServerName: "127.0.0.1", MinVersion: tls.VersionTLS12}
	err = untrusted.Send(context.Background(), EmailMessage{To: "awa@poro.test", Subject: "s", Body: "b"})
	require.ErrorIs(t, err, ErrEmailUnavailable, "an untrusted certificate stops the delivery")

	srv.rejectRcpt.Store(true)
	err = sender.Send(context.Background(), EmailMessage{To: "awa@poro.test", Subject: "s", Body: "b"})
	require.ErrorIs(t, err, ErrEmailUnavailable)
	srv.rejectRcpt.Store(false)

	closed := *sender
	closed.port = freePort(t)
	err = closed.Send(context.Background(), EmailMessage{To: "awa@poro.test", Subject: "s", Body: "b"})
	require.ErrorIs(t, err, ErrEmailUnavailable)
}

type receivedMail struct {
	tls  bool
	auth string
	from string
	to   []string
	data string
}

type fakeSMTP struct {
	ln         net.Listener
	cert       tls.Certificate
	pool       *x509.CertPool
	offerTLS   bool
	rejectRcpt atomic.Bool

	mu   sync.Mutex
	mail []receivedMail
}

// startFakeSMTP serves just enough SMTP for net/smtp: implicit TLS, or
// plain TCP that offers STARTTLS when offerTLS is set.
func startFakeSMTP(t *testing.T, implicit, offerTLS bool) *fakeSMTP {
	t.Helper()
	cert, pool := selfSignedCert(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := &fakeSMTP{ln: ln, cert: cert, pool: pool, offerTLS: offerTLS}
	if implicit {
		srv.ln = tls.NewListener(ln, &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
	}
	t.Cleanup(func() { _ = srv.ln.Close() })
	go func() {
		for {
			conn, err := srv.ln.Accept()
			if err != nil {
				return
			}
			go srv.serve(conn, implicit)
		}
	}()
	return srv
}

func (s *fakeSMTP) sender(t *testing.T, implicit bool) *smtpSender {
	t.Helper()
	port := s.ln.Addr().(*net.TCPAddr).Port
	return &smtpSender{
		host:        "127.0.0.1",
		port:        port,
		from:        &mail.Address{Name: "PORO", Address: "no-reply@poro.test"},
		tls:         &tls.Config{ServerName: "127.0.0.1", RootCAs: s.pool, MinVersion: tls.VersionTLS12},
		timeout:     5 * time.Second,
		implicitTLS: implicit,
	}
}

func (s *fakeSMTP) messages() []receivedMail {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]receivedMail(nil), s.mail...)
}

func (s *fakeSMTP) received(t *testing.T) receivedMail {
	t.Helper()
	msgs := s.messages()
	require.Len(t, msgs, 1)
	return msgs[0]
}

func (s *fakeSMTP) serve(conn net.Conn, secure bool) {
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	r := bufio.NewReader(conn)
	reply := func(line string) { _, _ = io.WriteString(conn, line+"\r\n") }
	var current receivedMail
	current.tls = secure
	reply("220 fake ESMTP")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.TrimRight(line, "\r\n")
		upper := strings.ToUpper(cmd)
		switch {
		case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
			if !current.tls && s.offerTLS {
				reply("250-fake")
				reply("250 STARTTLS")
			} else {
				reply("250-fake")
				reply("250 AUTH PLAIN")
			}
		case upper == "STARTTLS":
			reply("220 ready")
			tlsConn := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{s.cert}, MinVersion: tls.VersionTLS12})
			if err := tlsConn.Handshake(); err != nil {
				return
			}
			conn = tlsConn
			r = bufio.NewReader(conn)
			current.tls = true
		case strings.HasPrefix(upper, "AUTH PLAIN "):
			decoded, _ := base64.StdEncoding.DecodeString(cmd[len("AUTH PLAIN "):])
			current.auth = string(decoded)
			reply("235 ok")
		case strings.HasPrefix(upper, "MAIL FROM:"):
			current.from = strings.Fields(cmd[len("MAIL FROM:"):])[0]
			reply("250 ok")
		case strings.HasPrefix(upper, "RCPT TO:"):
			if s.rejectRcpt.Load() {
				reply("550 no such user")
				continue
			}
			current.to = append(current.to, strings.TrimSpace(cmd[len("RCPT TO:"):]))
			reply("250 ok")
		case upper == "DATA":
			reply("354 go ahead")
			var data strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				data.WriteString(l)
			}
			current.data = data.String()
			s.mu.Lock()
			s.mail = append(s.mail, current)
			s.mu.Unlock()
			reply("250 queued")
		case upper == "QUIT":
			reply("221 bye")
			return
		default:
			reply("250 ok")
		}
	}
}

func selfSignedCert(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "fake smtp"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	leaf, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, pool
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())
	return port
}
