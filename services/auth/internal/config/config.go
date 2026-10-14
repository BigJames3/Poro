package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

const (
	EnvDev     = "dev"
	EnvStaging = "staging"
	EnvProd    = "prod"

	// SMSProviderLog writes codes to the log instead of sending them. Dev only.
	SMSProviderLog = "log"
	// SMSProviderAfricasTalking sends codes through the Africa's Talking SMS API.
	SMSProviderAfricasTalking = "africastalking"
	// AfricasTalkingSandboxUser selects the Africa's Talking sandbox, which never reaches real phones.
	AfricasTalkingSandboxUser = "sandbox"

	// EmailProviderLog writes email codes to the log instead of sending them. Dev only.
	EmailProviderLog = "log"
	// EmailProviderSMTP sends email through an SMTP relay (Brevo, SES, ...) over TLS.
	EmailProviderSMTP = "smtp"

	minOTPSecretLen = 32
)

var callingCodePattern = regexp.MustCompile(`^\+[1-9][0-9]{0,3}$`)

// Config holds every environment setting for the auth service.
type Config struct {
	// App
	AppEnv         string   // dev, staging, prod
	AppPort        string   // 8081
	LogLevel       string   // debug, info, warn, error
	TrustedProxies []string // CIDRs or IPs allowed to set X-Forwarded-For

	// Postgres
	PostgresHost     string // localhost
	PostgresPort     string // 5433
	PostgresUser     string // poro
	PostgresPassword string
	PostgresDB       string // poro_auth
	PostgresSSLMode  string // disable

	// Redis
	RedisHost     string // localhost
	RedisPort     string // 6380
	RedisPassword string
	RedisDB       int // 0

	// JWT
	JWTPrivateKeyPath    string // ./keys/private.pem
	JWTPublicKeyPath     string // ./keys/public.pem
	JWTAccessTTL         string // 15m
	JWTRefreshTTL        string // 720h
	JWTRefreshReuseGrace string // 30s

	// OTP
	OtpTTL                 string   // 5m
	OtpMaxAttempts         int      // 3
	OtpSecret              string   // HMAC key for stored codes
	OtpRequestCooldown     string   // 60s
	OtpMaxRequestsPerHour  int      // 5
	OtpAllowedCallingCodes []string // +225,+221,+237,+234; empty allows every country
	SMSProvider            string   // log, africastalking

	// Africa's Talking
	AfricasTalkingUsername string // "sandbox" selects the sandbox
	AfricasTalkingAPIKey   string
	AfricasTalkingSenderID string // optional registered alphanumeric sender

	// Email
	EmailProvider        string // log, smtp
	EmailFrom            string // "PORO <no-reply@poro.africa>"
	EmailCodeTTL         string // 15m
	EmailCodeMaxAttempts int    // 5
	SMTPHost             string
	SMTPPort             int // 587 (STARTTLS) or 465 (implicit TLS)
	SMTPUsername         string
	SMTPPassword         string

	// Account lifecycle
	AccountDeletionGrace string // 720h: delay before a requested deletion erases the account
	AccountPurgeInterval string // 1h: how often due accounts are erased

	// Events and telemetry
	KafkaBrokers []string // localhost:9092
	OtelEndpoint string   // OTLP HTTP endpoint; empty disables trace export
}

// PostgresDSN returns a URL-encoded connection string. The password is escaped.
func (c *Config) PostgresDSN() string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.PostgresUser, c.PostgresPassword),
		Host:   net.JoinHostPort(c.PostgresHost, c.PostgresPort),
		Path:   "/" + c.PostgresDB,
	}
	q := u.Query()
	q.Set("sslmode", c.PostgresSSLMode)
	u.RawQuery = q.Encode()
	return u.String()
}

// RedisAddr returns host:port for the Redis client.
func (c *Config) RedisAddr() string {
	return net.JoinHostPort(c.RedisHost, c.RedisPort)
}

// IsDev reports whether the service runs in the local development environment.
func (c *Config) IsDev() bool {
	return c.AppEnv == EnvDev
}

// Validate rejects settings that are unsafe or unusable. Staging and prod
// must not rely on dev shortcuts such as the log SMS provider.
func (c *Config) Validate() error {
	var errs []error
	switch c.AppEnv {
	case EnvDev, EnvStaging, EnvProd:
	case "":
		// No default: a forgotten APP_ENV must not start a production
		// service with the shortcuts of local development.
		errs = append(errs, errors.New("APP_ENV is required: dev, staging or prod"))
	default:
		errs = append(errs, fmt.Errorf("APP_ENV must be dev, staging or prod, got %q", c.AppEnv))
	}
	if c.PostgresPassword == "" {
		errs = append(errs, errors.New("POSTGRES_PASSWORD is required"))
	}
	switch c.SMSProvider {
	case SMSProviderLog:
	case SMSProviderAfricasTalking:
		if c.AfricasTalkingUsername == "" || c.AfricasTalkingAPIKey == "" {
			errs = append(errs, errors.New("AFRICASTALKING_USERNAME and AFRICASTALKING_API_KEY are required"))
		}
		if c.AppEnv == EnvProd && c.AfricasTalkingUsername == AfricasTalkingSandboxUser {
			errs = append(errs, errors.New("the Africa's Talking sandbox is not allowed in prod"))
		}
	default:
		errs = append(errs, fmt.Errorf("SMS_PROVIDER %q is not supported", c.SMSProvider))
	}
	switch c.EmailProvider {
	case EmailProviderLog:
	case EmailProviderSMTP:
		if c.SMTPHost == "" || c.EmailFrom == "" {
			errs = append(errs, errors.New("SMTP_HOST and EMAIL_FROM are required with EMAIL_PROVIDER=smtp"))
		}
		if c.SMTPPort <= 0 || c.SMTPPort > 65535 {
			errs = append(errs, fmt.Errorf("SMTP_PORT must be a TCP port, got %d", c.SMTPPort))
		}
	default:
		errs = append(errs, fmt.Errorf("EMAIL_PROVIDER %q is not supported", c.EmailProvider))
	}
	if c.EmailCodeMaxAttempts <= 0 {
		errs = append(errs, errors.New("EMAIL_CODE_MAX_ATTEMPTS must be positive"))
	}
	for name, value := range map[string]string{
		"EMAIL_CODE_TTL":         c.EmailCodeTTL,
		"ACCOUNT_DELETION_GRACE": c.AccountDeletionGrace,
		"ACCOUNT_PURGE_INTERVAL": c.AccountPurgeInterval,
	} {
		if d, err := time.ParseDuration(value); err != nil || d <= 0 {
			errs = append(errs, fmt.Errorf("%s must be a positive duration, got %q", name, value))
		}
	}
	if len(c.KafkaBrokers) == 0 {
		errs = append(errs, errors.New("KAFKA_BROKERS is required"))
	}
	for _, code := range c.OtpAllowedCallingCodes {
		if !callingCodePattern.MatchString(code) {
			errs = append(errs, fmt.Errorf("OTP_ALLOWED_CALLING_CODES: %q is not a calling code like +225", code))
		}
	}
	if !c.IsDev() {
		if len(c.OtpSecret) < minOTPSecretLen {
			errs = append(errs, fmt.Errorf("OTP_HMAC_SECRET must be at least %d bytes outside dev", minOTPSecretLen))
		}
		if c.SMSProvider == SMSProviderLog {
			errs = append(errs, errors.New("SMS_PROVIDER=log is only allowed in dev"))
		}
		if c.EmailProvider == EmailProviderLog {
			errs = append(errs, errors.New("EMAIL_PROVIDER=log is only allowed in dev"))
		}
		switch c.PostgresSSLMode {
		case "require", "verify-ca", "verify-full":
		default:
			errs = append(errs, errors.New("POSTGRES_SSLMODE must be require, verify-ca or verify-full outside dev"))
		}
	}
	return errors.Join(errs...)
}

// Load reads configuration from the environment and an optional .env file,
// then validates it. Environment variables override the file. Missing .env is not an error.
func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load dotenv: %w", err)
	}

	v := viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	v.AutomaticEnv()
	setDefaults(v)

	if _, err := os.Stat(".env"); err == nil {
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("read config: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("stat config: %w", err)
	}

	cfg := &Config{
		AppEnv:         v.GetString("APP_ENV"),
		AppPort:        v.GetString("APP_PORT"),
		LogLevel:       v.GetString("LOG_LEVEL"),
		TrustedProxies: splitList(v.GetString("TRUSTED_PROXIES")),

		PostgresHost:     v.GetString("POSTGRES_HOST"),
		PostgresPort:     v.GetString("POSTGRES_PORT"),
		PostgresUser:     v.GetString("POSTGRES_USER"),
		PostgresPassword: v.GetString("POSTGRES_PASSWORD"),
		PostgresDB:       v.GetString("POSTGRES_DB"),
		PostgresSSLMode:  v.GetString("POSTGRES_SSLMODE"),

		RedisHost:     v.GetString("REDIS_HOST"),
		RedisPort:     v.GetString("REDIS_PORT"),
		RedisPassword: v.GetString("REDIS_PASSWORD"),
		RedisDB:       v.GetInt("REDIS_DB"),

		JWTPrivateKeyPath:    v.GetString("JWT_PRIVATE_KEY_PATH"),
		JWTPublicKeyPath:     v.GetString("JWT_PUBLIC_KEY_PATH"),
		JWTAccessTTL:         v.GetString("JWT_ACCESS_TTL"),
		JWTRefreshTTL:        v.GetString("JWT_REFRESH_TTL"),
		JWTRefreshReuseGrace: v.GetString("JWT_REFRESH_REUSE_GRACE"),

		OtpTTL:                 v.GetString("OTP_TTL"),
		OtpMaxAttempts:         v.GetInt("OTP_MAX_ATTEMPTS"),
		OtpSecret:              v.GetString("OTP_HMAC_SECRET"),
		OtpRequestCooldown:     v.GetString("OTP_REQUEST_COOLDOWN"),
		OtpMaxRequestsPerHour:  v.GetInt("OTP_MAX_REQUESTS_PER_HOUR"),
		OtpAllowedCallingCodes: splitList(v.GetString("OTP_ALLOWED_CALLING_CODES")),
		SMSProvider:            v.GetString("SMS_PROVIDER"),

		AfricasTalkingUsername: v.GetString("AFRICASTALKING_USERNAME"),
		AfricasTalkingAPIKey:   v.GetString("AFRICASTALKING_API_KEY"),
		AfricasTalkingSenderID: v.GetString("AFRICASTALKING_SENDER_ID"),

		EmailProvider:        v.GetString("EMAIL_PROVIDER"),
		EmailFrom:            v.GetString("EMAIL_FROM"),
		EmailCodeTTL:         v.GetString("EMAIL_CODE_TTL"),
		EmailCodeMaxAttempts: v.GetInt("EMAIL_CODE_MAX_ATTEMPTS"),
		SMTPHost:             v.GetString("SMTP_HOST"),
		SMTPPort:             v.GetInt("SMTP_PORT"),
		SMTPUsername:         v.GetString("SMTP_USERNAME"),
		SMTPPassword:         v.GetString("SMTP_PASSWORD"),

		AccountDeletionGrace: v.GetString("ACCOUNT_DELETION_GRACE"),
		AccountPurgeInterval: v.GetString("ACCOUNT_PURGE_INTERVAL"),

		KafkaBrokers: splitList(v.GetString("KAFKA_BROKERS")),
		OtelEndpoint: v.GetString("OTEL_EXPORTER_OTLP_ENDPOINT"),
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("APP_PORT", "8081")
	v.SetDefault("LOG_LEVEL", "debug")
	v.SetDefault("TRUSTED_PROXIES", "")

	v.SetDefault("POSTGRES_HOST", "localhost")
	v.SetDefault("POSTGRES_PORT", "5433")
	v.SetDefault("POSTGRES_USER", "poro")
	v.SetDefault("POSTGRES_DB", "poro_auth")
	v.SetDefault("POSTGRES_SSLMODE", "disable")

	v.SetDefault("REDIS_HOST", "localhost")
	v.SetDefault("REDIS_PORT", "6380")
	v.SetDefault("REDIS_PASSWORD", "")
	v.SetDefault("REDIS_DB", 0)

	v.SetDefault("JWT_PRIVATE_KEY_PATH", "./keys/private.pem")
	v.SetDefault("JWT_PUBLIC_KEY_PATH", "./keys/public.pem")
	v.SetDefault("JWT_ACCESS_TTL", "15m")
	v.SetDefault("JWT_REFRESH_TTL", "720h")
	v.SetDefault("JWT_REFRESH_REUSE_GRACE", "30s")

	v.SetDefault("OTP_TTL", "5m")
	v.SetDefault("OTP_MAX_ATTEMPTS", 3)
	v.SetDefault("OTP_HMAC_SECRET", "")
	v.SetDefault("OTP_REQUEST_COOLDOWN", "60s")
	v.SetDefault("OTP_MAX_REQUESTS_PER_HOUR", 5)
	v.SetDefault("OTP_ALLOWED_CALLING_CODES", "+225,+221,+237,+234")
	v.SetDefault("SMS_PROVIDER", SMSProviderLog)

	v.SetDefault("AFRICASTALKING_USERNAME", "")
	v.SetDefault("AFRICASTALKING_API_KEY", "")
	v.SetDefault("AFRICASTALKING_SENDER_ID", "")

	v.SetDefault("EMAIL_PROVIDER", EmailProviderLog)
	v.SetDefault("EMAIL_FROM", "")
	v.SetDefault("EMAIL_CODE_TTL", "15m")
	v.SetDefault("EMAIL_CODE_MAX_ATTEMPTS", 5)
	v.SetDefault("SMTP_HOST", "")
	v.SetDefault("SMTP_PORT", 587)
	v.SetDefault("SMTP_USERNAME", "")
	v.SetDefault("SMTP_PASSWORD", "")

	v.SetDefault("ACCOUNT_DELETION_GRACE", "720h")
	v.SetDefault("ACCOUNT_PURGE_INTERVAL", "1h")

	v.SetDefault("KAFKA_BROKERS", "localhost:9092")
	v.SetDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "")
}

func splitList(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
