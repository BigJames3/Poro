package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

const (
	EnvDev     = "dev"
	EnvStaging = "staging"
	EnvProd    = "prod"

	// SMSProviderLog writes codes to the log instead of sending them. Dev only.
	SMSProviderLog = "log"

	minOTPSecretLen = 32
)

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
	OtpTTL                string // 5m
	OtpMaxAttempts        int    // 3
	OtpSecret             string // HMAC key for stored codes
	OtpRequestCooldown    string // 60s
	OtpMaxRequestsPerHour int    // 5
	SMSProvider           string // log
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
	default:
		errs = append(errs, fmt.Errorf("APP_ENV must be dev, staging or prod, got %q", c.AppEnv))
	}
	if c.SMSProvider != SMSProviderLog {
		errs = append(errs, fmt.Errorf("SMS_PROVIDER %q is not supported", c.SMSProvider))
	}
	if !c.IsDev() {
		if len(c.OtpSecret) < minOTPSecretLen {
			errs = append(errs, fmt.Errorf("OTP_HMAC_SECRET must be at least %d bytes outside dev", minOTPSecretLen))
		}
		if c.SMSProvider == SMSProviderLog {
			errs = append(errs, errors.New("SMS_PROVIDER=log is only allowed in dev"))
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

		OtpTTL:                v.GetString("OTP_TTL"),
		OtpMaxAttempts:        v.GetInt("OTP_MAX_ATTEMPTS"),
		OtpSecret:             v.GetString("OTP_HMAC_SECRET"),
		OtpRequestCooldown:    v.GetString("OTP_REQUEST_COOLDOWN"),
		OtpMaxRequestsPerHour: v.GetInt("OTP_MAX_REQUESTS_PER_HOUR"),
		SMSProvider:           v.GetString("SMS_PROVIDER"),
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("APP_ENV", EnvDev)
	v.SetDefault("APP_PORT", "8081")
	v.SetDefault("LOG_LEVEL", "debug")
	v.SetDefault("TRUSTED_PROXIES", "")

	v.SetDefault("POSTGRES_HOST", "localhost")
	v.SetDefault("POSTGRES_PORT", "5433")
	v.SetDefault("POSTGRES_USER", "poro")
	v.SetDefault("POSTGRES_PASSWORD", "poro_dev_password")
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
	v.SetDefault("SMS_PROVIDER", SMSProviderLog)
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
