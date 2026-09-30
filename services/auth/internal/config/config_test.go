package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	prod := func() *Config {
		return &Config{
			AppEnv:          EnvProd,
			SMSProvider:     "log",
			OtpSecret:       "0123456789abcdef0123456789abcdef",
			PostgresSSLMode: "verify-full",
			KafkaBrokers:    []string{"redpanda:9092"},
		}
	}

	require.NoError(t, (&Config{AppEnv: EnvDev, SMSProvider: SMSProviderLog, PostgresSSLMode: "disable", KafkaBrokers: []string{"localhost:9092"}}).Validate())

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
	t.Setenv("TRUSTED_PROXIES", " 10.0.0.0/8, ,172.16.0.1 ")
	t.Setenv("OTP_MAX_REQUESTS_PER_HOUR", "7")

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, []string{"10.0.0.0/8", "172.16.0.1"}, cfg.TrustedProxies)
	require.Equal(t, 7, cfg.OtpMaxRequestsPerHour)
	require.Equal(t, "30s", cfg.JWTRefreshReuseGrace)
	require.Equal(t, []string{"localhost:9092"}, cfg.KafkaBrokers)
	require.Equal(t, []string{"+225", "+221", "+237", "+234"}, cfg.OtpAllowedCallingCodes)
	require.Equal(t, "postgres://poro:poro_dev_password@localhost:5433/poro_auth?sslmode=disable", cfg.PostgresDSN())

	t.Setenv("APP_ENV", EnvProd)
	_, err = Load()
	require.Error(t, err, "prod without a real SMS provider or secrets must not start")
}
