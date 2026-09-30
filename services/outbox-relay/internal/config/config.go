// Package config loads the outbox relay settings from the environment.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds the relay settings. One relay serves one service database.
type Config struct {
	AppEnv   string // dev, staging, prod
	Source   string // service whose outbox is relayed, e.g. auth
	HTTPPort string // health and metrics
	LogLevel string

	PostgresHost     string
	PostgresPort     string
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string
	PostgresSSLMode  string

	KafkaBrokers []string

	BatchSize    int
	PollInterval time.Duration
	Retention    time.Duration
}

// IsDev reports whether the relay runs on a developer machine.
func (c *Config) IsDev() bool { return c.AppEnv == "dev" }

// PostgresDSN returns a URL-encoded connection string.
func (c *Config) PostgresDSN() string {
	u := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.PostgresUser, c.PostgresPassword),
		Host:     net.JoinHostPort(c.PostgresHost, c.PostgresPort),
		Path:     "/" + c.PostgresDB,
		RawQuery: url.Values{"sslmode": {c.PostgresSSLMode}}.Encode(),
	}
	return u.String()
}

// Validate rejects unusable or unsafe settings.
func (c *Config) Validate() error {
	var errs []error
	switch c.AppEnv {
	case "dev", "staging", "prod":
	default:
		errs = append(errs, fmt.Errorf("APP_ENV must be dev, staging or prod, got %q", c.AppEnv))
	}
	if c.Source == "" {
		errs = append(errs, errors.New("RELAY_SOURCE is required"))
	}
	if c.PostgresDB == "" {
		errs = append(errs, errors.New("POSTGRES_DB is required"))
	}
	if len(c.KafkaBrokers) == 0 {
		errs = append(errs, errors.New("KAFKA_BROKERS is required"))
	}
	if c.BatchSize <= 0 || c.BatchSize > 1000 {
		errs = append(errs, errors.New("RELAY_BATCH_SIZE must be between 1 and 1000"))
	}
	if c.PollInterval <= 0 {
		errs = append(errs, errors.New("RELAY_POLL_INTERVAL must be positive"))
	}
	if c.Retention <= 0 {
		errs = append(errs, errors.New("RELAY_RETENTION must be positive"))
	}
	if !c.IsDev() {
		switch c.PostgresSSLMode {
		case "require", "verify-ca", "verify-full":
		default:
			errs = append(errs, errors.New("POSTGRES_SSLMODE must be require, verify-ca or verify-full outside dev"))
		}
	}
	return errors.Join(errs...)
}

// Load reads the environment and validates it.
func Load() (*Config, error) {
	v := viper.New()
	v.AutomaticEnv()
	v.SetDefault("APP_ENV", "dev")
	v.SetDefault("HTTP_PORT", "9100")
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("POSTGRES_HOST", "localhost")
	v.SetDefault("POSTGRES_PORT", "5433")
	v.SetDefault("POSTGRES_USER", "poro")
	v.SetDefault("POSTGRES_PASSWORD", "")
	v.SetDefault("POSTGRES_SSLMODE", "disable")
	v.SetDefault("KAFKA_BROKERS", "localhost:9092")
	v.SetDefault("RELAY_BATCH_SIZE", 100)
	v.SetDefault("RELAY_POLL_INTERVAL", "500ms")
	v.SetDefault("RELAY_RETENTION", "72h")

	poll, err := time.ParseDuration(v.GetString("RELAY_POLL_INTERVAL"))
	if err != nil {
		return nil, fmt.Errorf("RELAY_POLL_INTERVAL: %w", err)
	}
	retention, err := time.ParseDuration(v.GetString("RELAY_RETENTION"))
	if err != nil {
		return nil, fmt.Errorf("RELAY_RETENTION: %w", err)
	}
	cfg := &Config{
		AppEnv:           v.GetString("APP_ENV"),
		Source:           v.GetString("RELAY_SOURCE"),
		HTTPPort:         v.GetString("HTTP_PORT"),
		LogLevel:         v.GetString("LOG_LEVEL"),
		PostgresHost:     v.GetString("POSTGRES_HOST"),
		PostgresPort:     v.GetString("POSTGRES_PORT"),
		PostgresUser:     v.GetString("POSTGRES_USER"),
		PostgresPassword: v.GetString("POSTGRES_PASSWORD"),
		PostgresDB:       v.GetString("POSTGRES_DB"),
		PostgresSSLMode:  v.GetString("POSTGRES_SSLMODE"),
		KafkaBrokers:     splitList(v.GetString("KAFKA_BROKERS")),
		BatchSize:        v.GetInt("RELAY_BATCH_SIZE"),
		PollInterval:     poll,
		Retention:        retention,
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return cfg, nil
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
