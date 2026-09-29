package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config holds every environment setting for the auth service.
type Config struct {
	// App
	AppEnv   string // dev, staging, prod
	AppPort  string // 8081
	LogLevel string // debug, info, warn, error

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
	JWTPrivateKeyPath string // ./keys/private.pem
	JWTPublicKeyPath  string // ./keys/public.pem
	JWTAccessTTL      string // 15m
	JWTRefreshTTL     string // 720h

	// OTP
	OtpTTL         string // 5m
	OtpMaxAttempts int    // 3
	OtpProviderURL string
	OtpProviderKey string
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

// Load reads configuration from the environment and an optional .env file.
// Environment variables override the file. Missing .env is not an error.
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
		AppEnv:   v.GetString("APP_ENV"),
		AppPort:  v.GetString("APP_PORT"),
		LogLevel: v.GetString("LOG_LEVEL"),

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

		JWTPrivateKeyPath: v.GetString("JWT_PRIVATE_KEY_PATH"),
		JWTPublicKeyPath:  v.GetString("JWT_PUBLIC_KEY_PATH"),
		JWTAccessTTL:      v.GetString("JWT_ACCESS_TTL"),
		JWTRefreshTTL:     v.GetString("JWT_REFRESH_TTL"),

		OtpTTL:         v.GetString("OTP_TTL"),
		OtpMaxAttempts: v.GetInt("OTP_MAX_ATTEMPTS"),
		OtpProviderURL: v.GetString("OTP_PROVIDER_URL"),
		OtpProviderKey: v.GetString("OTP_PROVIDER_KEY"),
	}

	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("APP_ENV", "dev")
	v.SetDefault("APP_PORT", "8081")
	v.SetDefault("LOG_LEVEL", "debug")

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

	v.SetDefault("OTP_TTL", "5m")
	v.SetDefault("OTP_MAX_ATTEMPTS", 3)
	v.SetDefault("OTP_PROVIDER_URL", "")
	v.SetDefault("OTP_PROVIDER_KEY", "")
}
