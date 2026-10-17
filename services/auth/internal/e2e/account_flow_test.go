package e2e

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/shared-go/health"
	"github.com/poro/shared-go/httpx"
	"github.com/poro/shared-go/revocation"

	"github.com/poro/auth/internal/config"
	"github.com/poro/auth/internal/handler"
	"github.com/poro/auth/internal/middleware"
	"github.com/poro/auth/internal/repository"
	"github.com/poro/auth/internal/routes"
	"github.com/poro/auth/internal/service"
)

var codePattern = regexp.MustCompile(`\b[0-9]{6}\b`)

// mailbox records the emails auth sends.
type mailbox struct {
	mu   sync.Mutex
	sent []service.EmailMessage
	got  chan struct{}
}

func (m *mailbox) Send(_ context.Context, msg service.EmailMessage) error {
	m.mu.Lock()
	m.sent = append(m.sent, msg)
	m.mu.Unlock()
	m.got <- struct{}{}
	return nil
}

// code waits for the next email and reads its code.
func (m *mailbox) code(t *testing.T) (service.EmailMessage, string) {
	t.Helper()
	select {
	case <-m.got:
	case <-time.After(10 * time.Second):
		t.Fatal("no email sent")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	msg := m.sent[len(m.sent)-1]
	code := codePattern.FindString(msg.Body)
	require.NotEmpty(t, code)
	return msg, code
}

func (m *mailbox) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sent)
}

// TestEmailAccountLifecycle drives the HTTP API end to end: sign-up, email
// verification, sessions, forgotten password, and a cancelled deletion.
// startAuthAPI wires the auth HTTP API on a real Postgres and an in-memory
// Redis, with every limit at its production default.
func startAuthAPI(t *testing.T) (*client, *mailbox, *miniredis.Miniredis) {
	t.Helper()
	pool := startPostgres(t)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	log := zap.NewNop()

	cfg := &config.Config{
		AppEnv: config.EnvDev, JWTAccessTTL: "15m", JWTRefreshTTL: "720h", JWTRefreshReuseGrace: "30s",
		OtpTTL: "5m", OtpMaxAttempts: 3, OtpRequestCooldown: "60s", OtpMaxRequestsPerHour: 5,
		EmailCodeTTL: "15m", EmailCodeMaxAttempts: 5,
		SMSDailyLimitPerPhone: 10, SMSHourlyLimitPerPrefix: 20, SMSHourlyLimitPerCountry: 1000, SMSDailyLimit: 5000,
		LoginMaxFailures: 5, LoginFailureWindow: "15m", LoginLockBase: "1m", LoginLockMax: "15m",
	}
	cfg.JWTPrivateKeyPath, cfg.JWTPublicKeyPath = writeKeys(t)
	registry, err := revocation.Open("redis://" + mr.Addr() + "/15")
	require.NoError(t, err)
	t.Cleanup(func() { _ = registry.Close() })
	tokens, err := service.NewTokenService(cfg, registry)
	require.NoError(t, err)
	otpThrottle, err := service.NewRedisOTPThrottle(cfg, rdb)
	require.NoError(t, err)
	emailThrottle, err := service.NewRedisEmailThrottle(cfg, rdb)
	require.NoError(t, err)
	users := repository.NewUserRepository(pool)
	refresh := repository.NewRefreshTokenRepository(pool)
	smsBudget, err := service.NewRedisSMSBudget(cfg, rdb, nil)
	require.NoError(t, err)
	logins, err := service.NewRedisLoginGuard(cfg, rdb)
	require.NoError(t, err)
	otp, err := service.NewOTPService(cfg, repository.NewOTPRepository(pool), otpThrottle, smsBudget, noSMS{}, log)
	require.NoError(t, err)
	authSvc, err := service.NewAuthService(cfg, users, refresh, otp, tokens, logins, log)
	require.NoError(t, err)
	accounts, err := service.NewAccountService(users, refresh, otp, tokens, logins, 720*time.Hour, log)
	require.NoError(t, err)
	mails := &mailbox{got: make(chan struct{}, 10)}
	emails, err := service.NewEmailService(cfg, users, repository.NewEmailCodeRepository(pool), emailThrottle, mails, accounts, logins, log)
	require.NoError(t, err)

	// As behind the gateway: the client IP comes from X-Forwarded-For.
	app := fiber.New(fiber.Config{
		ErrorHandler:            httpx.ErrorHandler(log),
		ProxyHeader:             fiber.HeaderXForwardedFor,
		EnableTrustedProxyCheck: true,
		TrustedProxies:          []string{"0.0.0.0"},
	})
	app.Use(httpx.RequestID())
	routes.Register(app, routes.Dependencies{
		Auth:     handler.NewAuthHandler(authSvc).SendVerificationOnSignup(emails, log),
		Account:  handler.NewAccountHandler(accounts),
		Email:    handler.NewEmailHandler(emails),
		Health:   health.New("auth", "test", map[string]health.Check{}),
		Metrics:  func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) },
		JWKS:     handler.NewJWKSHandler(tokens),
		Tokens:   tokens,
		Accounts: users,
		Limiter:  middleware.NewLimiter(rdb),
	})
	return &client{t: t, app: app}, mails, mr
}

func TestEmailAccountLifecycle(t *testing.T) {
	api, mails, mr := startAuthAPI(t)

	// Sign-up sends the verification email in the background.
	status, body := api.call(http.MethodPost, "/api/v1/auth/email/register", "",
		`{"email":"Awa.Kone@Poro.test","password":"premier-mdp","full_name":"Awa Koné","device_name":"Tecno Spark 20"}`)
	require.Equal(t, http.StatusCreated, status)
	access := body["access_token"].(string)
	firstRefresh := body["refresh_token"].(string)
	msg, code := mails.code(t)
	require.Equal(t, "awa.kone@poro.test", msg.To)
	require.Equal(t, "Ton code de vérification PORO", msg.Subject)

	status, body = api.call(http.MethodGet, "/api/v1/auth/me", access, "")
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, false, body["email_verified"])

	status, body = api.call(http.MethodPost, "/api/v1/auth/email/verification/confirm", access, `{"code":"`+other(code)+`"}`)
	require.Equal(t, http.StatusUnprocessableEntity, status)
	require.Equal(t, "code_invalid", body["code"])
	status, body = api.call(http.MethodPost, "/api/v1/auth/email/verification/confirm", access, `{"code":"`+code+`"}`)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, true, body["email_verified"])
	status, body = api.call(http.MethodPost, "/api/v1/auth/email/verification/request", access, "")
	require.Equal(t, http.StatusConflict, status)
	require.Equal(t, "email_already_verified", body["code"])

	status, list := api.list(http.MethodGet, "/api/v1/auth/sessions", access)
	require.Equal(t, http.StatusOK, status)
	require.Len(t, list, 1)
	require.Equal(t, "Tecno Spark 20", list[0]["device"])
	require.Equal(t, true, list[0]["current"])

	// "Sign out other devices" refuses their access tokens at once, not only
	// their refresh tokens.
	status, body = api.call(http.MethodPost, "/api/v1/auth/email/login", "",
		`{"email":"awa.kone@poro.test","password":"premier-mdp","device_name":"Itel A70"}`)
	require.Equal(t, http.StatusOK, status)
	secondDevice := body["access_token"].(string)
	status, _ = api.call(http.MethodGet, "/api/v1/auth/me", secondDevice, "")
	require.Equal(t, http.StatusOK, status)
	status, body = api.call(http.MethodPost, "/api/v1/auth/sessions/revoke-others", access, "")
	require.Equal(t, http.StatusOK, status)
	require.EqualValues(t, 1, body["revoked"])
	status, _ = api.call(http.MethodGet, "/api/v1/auth/me", secondDevice, "")
	require.Equal(t, http.StatusUnauthorized, status, "the other device's access token is refused")
	status, _ = api.call(http.MethodGet, "/api/v1/auth/me", access, "")
	require.Equal(t, http.StatusOK, status, "the caller stays signed in")

	// Forgotten password: an unknown address gets the same answer and no email.
	sentBefore := mails.count()
	status, _ = api.call(http.MethodPost, "/api/v1/auth/email/password/forgot", "", `{"email":"nobody@poro.test"}`)
	require.Equal(t, http.StatusAccepted, status)
	require.Equal(t, sentBefore, mails.count())

	status, body = api.call(http.MethodPost, "/api/v1/auth/email/password/forgot", "", `{"email":"awa.kone@poro.test"}`)
	require.Equal(t, http.StatusTooManyRequests, status, "one cooldown per address, whatever the code")
	require.Equal(t, "email_throttled", body["code"])
	mr.FastForward(61 * time.Second)
	status, _ = api.call(http.MethodPost, "/api/v1/auth/email/password/forgot", "", `{"email":"awa.kone@poro.test"}`)
	require.Equal(t, http.StatusAccepted, status)
	msg, code = mails.code(t)
	require.Equal(t, "Réinitialise ton mot de passe PORO", msg.Subject)
	status, _ = api.call(http.MethodPost, "/api/v1/auth/email/password/reset", "",
		`{"email":"awa.kone@poro.test","code":"`+code+`","new_password":"second-mdp"}`)
	require.Equal(t, http.StatusNoContent, status)

	status, body = api.call(http.MethodPost, "/api/v1/auth/refresh", "", `{"refresh_token":"`+firstRefresh+`"}`)
	require.Equal(t, http.StatusUnauthorized, status, "the reset signed every device out")
	require.Equal(t, "refresh_token_invalid", body["code"])
	status, _ = api.call(http.MethodGet, "/api/v1/auth/me", access, "")
	require.Equal(t, http.StatusUnauthorized, status, "including the access tokens issued before the reset")
	status, _ = api.call(http.MethodPost, "/api/v1/auth/email/login", "", `{"email":"awa.kone@poro.test","password":"premier-mdp"}`)
	require.Equal(t, http.StatusUnauthorized, status)
	status, body = api.call(http.MethodPost, "/api/v1/auth/email/login", "", `{"email":"awa.kone@poro.test","password":"second-mdp"}`)
	require.Equal(t, http.StatusOK, status)
	access = body["access_token"].(string)

	// A deletion request is cancelled by signing in again.
	status, body = api.call(http.MethodPost, "/api/v1/auth/me/delete", access, `{"password":"second-mdp"}`)
	require.Equal(t, http.StatusAccepted, status)
	require.NotEmpty(t, body["deletion_scheduled_at"])
	status, _ = api.call(http.MethodGet, "/api/v1/auth/me", access, "")
	require.Equal(t, http.StatusUnauthorized, status, "a deletion request signs every device out at once")
	status, body = api.call(http.MethodPost, "/api/v1/auth/email/login", "", `{"email":"awa.kone@poro.test","password":"second-mdp"}`)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, true, body["deletion_cancelled"])
	user := body["user"].(map[string]any)
	require.Nil(t, user["deletion_scheduled_at"])
	require.Equal(t, true, user["email_verified"])
}

type client struct {
	t   *testing.T
	app *fiber.App
	ip  string // sent as X-Forwarded-For when set
}

// call returns the status and the data object, or the error object on failure.
func (c *client) call(method, path, token, payload string) (int, map[string]any) {
	c.t.Helper()
	status, raw := c.do(method, path, token, payload)
	if len(raw) == 0 {
		return status, nil
	}
	var env struct {
		Data  map[string]any `json:"data"`
		Error map[string]any `json:"error"`
	}
	require.NoError(c.t, json.Unmarshal(raw, &env))
	if env.Error != nil {
		return status, env.Error
	}
	return status, env.Data
}

func (c *client) list(method, path, token string) (int, []map[string]any) {
	c.t.Helper()
	status, raw := c.do(method, path, token, "")
	var env struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(c.t, json.Unmarshal(raw, &env))
	return status, env.Data
}

func (c *client) do(method, path, token, payload string) (int, []byte) {
	c.t.Helper()
	status, _, raw := c.send(method, path, token, payload)
	return status, raw
}

// send returns the status, the headers and the body of the response.
func (c *client) send(method, path, token, payload string) (int, http.Header, []byte) {
	c.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(payload))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderUserAgent, "PoroAndroid/1.0")
	if token != "" {
		req.Header.Set(fiber.HeaderAuthorization, "Bearer "+token)
	}
	if c.ip != "" {
		req.Header.Set(fiber.HeaderXForwardedFor, c.ip)
	}
	resp, err := c.app.Test(req, -1)
	require.NoError(c.t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(c.t, err)
	return resp.StatusCode, resp.Header, raw
}

func other(code string) string {
	if code == "000000" {
		return "111111"
	}
	return "000000"
}

// writeKeys writes a fresh RSA pair, as scripts/generate-keys.sh would.
func writeKeys(t *testing.T) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	public, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	dir := t.TempDir()
	privatePath, publicPath := filepath.Join(dir, "private.pem"), filepath.Join(dir, "public.pem")
	require.NoError(t, os.WriteFile(privatePath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0o600))
	require.NoError(t, os.WriteFile(publicPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: public}), 0o600))
	return privatePath, publicPath
}

type noSMS struct{}

func (noSMS) SendOTP(context.Context, string, string, time.Duration) error { return nil }
