package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	prod := func() *Config {
		return &Config{
			PostgresPassword: "secret",
			AppEnv:           EnvProd,
			SMSProvider:      "log",
			OtpSecret:        "0123456789abcdef0123456789abcdef",
			PostgresSSLMode:  "verify-full",
			KafkaBrokers:     []string{"redpanda:9092"},

			AccountDeletionGrace: "720h",
			AccountPurgeInterval: "1h",

			EmailProvider:        EmailProviderSMTP,
			EmailFrom:            "PORO <no-reply@poro.test>",
			EmailCodeTTL:         "15m",
			EmailCodeMaxAttempts: 5,
			SMTPHost:             "smtp.poro.test",
			SMTPPort:             587,
		}
	}

	require.NoError(t, (&Config{
		AppEnv: EnvDev, PostgresPassword: "secret", SMSProvider: SMSProviderLog, PostgresSSLMode: "disable", KafkaBrokers: []string{"localhost:9092"},
		AccountDeletionGrace: "720h", AccountPurgeInterval: "1h",
		EmailProvider: EmailProviderLog, EmailCodeTTL: "15m", EmailCodeMaxAttempts: 5,
	}).Validate())

	valid := prod()
	valid.SMSProvider = SMSProviderAfricasTalking
	valid.AfricasTalkingUsername = "poro"
	valid.AfricasTalkingAPIKey = "k"
	valid.OtpAllowedCallingCodes = []string{"+225", "+221", "+237", "+234"}
	require.NoError(t, valid.Validate(), "prod with Africa's Talking starts")

	staging := *valid
	staging.AppEnv = EnvStaging
	staging.AfricasTalkingUsername = AfricasTalkingSandboxUser
	require.NoError(t, staging.Validate(), "staging may use the sandbox")

	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{name: "unknown env", mutate: func(c *Config) { c.AppEnv = "production" }, want: "APP_ENV"},
		{name: "log sms outside dev", mutate: func(*Config) {}, want: "only allowed in dev"},
		{name: "unsupported sms", mutate: func(c *Config) { c.SMSProvider = "twilio" }, want: "not supported"},
		{name: "short otp secret", mutate: func(c *Config) { c.OtpSecret = "short" }, want: "OTP_HMAC_SECRET"},
		{name: "plaintext postgres", mutate: func(c *Config) { c.PostgresSSLMode = "disable" }, want: "POSTGRES_SSLMODE"},
		{name: "africastalking without credentials", mutate: func(c *Config) { c.SMSProvider = SMSProviderAfricasTalking }, want: "AFRICASTALKING_API_KEY"},
		{name: "africastalking sandbox in prod", mutate: func(c *Config) {
			c.SMSProvider = SMSProviderAfricasTalking
			c.AfricasTalkingUsername = AfricasTalkingSandboxUser
			c.AfricasTalkingAPIKey = "k"
		}, want: "sandbox"},
		{name: "no kafka brokers", mutate: func(c *Config) { c.KafkaBrokers = nil }, want: "KAFKA_BROKERS"},
		{name: "no deletion grace", mutate: func(c *Config) { c.AccountDeletionGrace = "" }, want: "ACCOUNT_DELETION_GRACE"},
		{name: "negative deletion grace", mutate: func(c *Config) { c.AccountDeletionGrace = "-1h" }, want: "ACCOUNT_DELETION_GRACE"},
		{name: "bad purge interval", mutate: func(c *Config) { c.AccountPurgeInterval = "hourly" }, want: "ACCOUNT_PURGE_INTERVAL"},
		{name: "log email outside dev", mutate: func(c *Config) { c.EmailProvider = EmailProviderLog }, want: "EMAIL_PROVIDER=log"},
		{name: "unsupported email", mutate: func(c *Config) { c.EmailProvider = "sendmail" }, want: "EMAIL_PROVIDER"},
		{name: "smtp without host", mutate: func(c *Config) { c.SMTPHost = "" }, want: "SMTP_HOST"},
		{name: "smtp without sender", mutate: func(c *Config) { c.EmailFrom = "" }, want: "EMAIL_FROM"},
		{name: "smtp bad port", mutate: func(c *Config) { c.SMTPPort = 0 }, want: "SMTP_PORT"},
		{name: "bad email code ttl", mutate: func(c *Config) { c.EmailCodeTTL = "soon" }, want: "EMAIL_CODE_TTL"},
		{name: "no email attempts", mutate: func(c *Config) { c.EmailCodeMaxAttempts = 0 }, want: "EMAIL_CODE_MAX_ATTEMPTS"},
		{name: "bad calling code", mutate: func(c *Config) { c.OtpAllowedCallingCodes = []string{"+225", "225"} }, want: "OTP_ALLOWED_CALLING_CODES"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := prod()
			tc.mutate(cfg)
			require.ErrorContains(t, cfg.Validate(), tc.want)
		})
	}
}

func TestLoadReadsEnvironment(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(t.TempDir()))
	t.Cleanup(func() { _ = os.Chdir(wd) })
	t.Setenv("APP_ENV", EnvDev)
	t.Setenv("POSTGRES_PASSWORD", "poro_dev_password")
	t.Setenv("TRUSTED_PROXIES", " 10.0.0.0/8, ,172.16.0.1 ")
	t.Setenv("OTP_MAX_REQUESTS_PER_HOUR", "7")

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, []string{"10.0.0.0/8", "172.16.0.1"}, cfg.TrustedProxies)
	require.Equal(t, 7, cfg.OtpMaxRequestsPerHour)
	require.Equal(t, "30s", cfg.JWTRefreshReuseGrace)
	require.Equal(t, "720h", cfg.AccountDeletionGrace)
	require.Equal(t, EmailProviderLog, cfg.EmailProvider)
	require.Equal(t, "15m", cfg.EmailCodeTTL)
	require.Equal(t, 5, cfg.EmailCodeMaxAttempts)
	require.Equal(t, 587, cfg.SMTPPort)
	require.Equal(t, "1h", cfg.AccountPurgeInterval)
	require.Equal(t, []string{"localhost:9092"}, cfg.KafkaBrokers)
	require.Equal(t, []string{"+225", "+221", "+237", "+234"}, cfg.OtpAllowedCallingCodes)
	require.Equal(t, "postgres://poro:poro_dev_password@localhost:5433/poro_auth?sslmode=disable", cfg.PostgresDSN())

	t.Setenv("APP_ENV", EnvProd)
	_, err = Load()
	require.Error(t, err, "prod without a real SMS provider or secrets must not start")
}

// A forgotten APP_ENV or database password stops the service instead of
// starting it with the shortcuts of local development.
func TestLoadRequiresAppEnvAndSecrets(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("APP_ENV", "")
	t.Setenv("POSTGRES_PASSWORD", "")
	_, err := Load()
	require.ErrorContains(t, err, "APP_ENV is required")
	require.ErrorContains(t, err, "POSTGRES_PASSWORD is required")
}
