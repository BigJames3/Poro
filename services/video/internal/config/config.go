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
)

// Config holds every environment setting for the video API and media-worker.
type Config struct {
	AppEnv         string
	AppPort        string
	LogLevel       string
	TrustedProxies []string

	PostgresHost     string
	PostgresPort     string
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string
	PostgresSSLMode  string

	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int

	JWKSURL      string
	KafkaBrokers []string
	OtelEndpoint string

	S3Endpoint       string
	S3PublicEndpoint string
	S3Region         string
	S3Bucket         string
	S3AccessKey      string
	S3SecretKey      string
	S3ForcePathStyle bool

	FFmpegBin  string
	FFprobeBin string
}

// PostgresDSN returns a URL-encoded connection string.
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

// PublicObjectURL is the client-reachable URL of an object key.
func (c *Config) PublicObjectURL(key string) string {
	base := strings.TrimRight(c.S3PublicEndpoint, "/")
	return base + "/" + c.S3Bucket + "/" + strings.TrimLeft(key, "/")
}

// IsDev reports whether the service runs in the local development environment.
func (c *Config) IsDev() bool {
	return c.AppEnv == EnvDev
}

// Validate rejects settings that are unsafe or unusable.
func (c *Config) Validate() error {
	var errs []error
	switch c.AppEnv {
	case EnvDev, EnvStaging, EnvProd:
	default:
		errs = append(errs, fmt.Errorf("APP_ENV must be dev, staging or prod, got %q", c.AppEnv))
	}
	if c.JWKSURL == "" {
		errs = append(errs, errors.New("JWKS_URL is required"))
	}
	if len(c.KafkaBrokers) == 0 {
		errs = append(errs, errors.New("KAFKA_BROKERS is required"))
	}
	if c.S3Endpoint == "" || c.S3Bucket == "" {
		errs = append(errs, errors.New("S3_ENDPOINT and S3_BUCKET are required"))
	}
	if c.S3PublicEndpoint == "" {
		errs = append(errs, errors.New("S3_PUBLIC_ENDPOINT is required"))
	}
	if !c.IsDev() {
		if c.S3AccessKey == "" || c.S3SecretKey == "" {
			errs = append(errs, errors.New("S3 credentials are required outside dev"))
		}
		switch c.PostgresSSLMode {
		case "require", "verify-ca", "verify-full":
		default:
			errs = append(errs, errors.New("POSTGRES_SSLMODE must be require, verify-ca or verify-full outside dev"))
		}
	}
	return errors.Join(errs...)
}

// Load reads configuration from the environment and an optional .env file.
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

		JWKSURL:      v.GetString("JWKS_URL"),
		KafkaBrokers: splitList(v.GetString("KAFKA_BROKERS")),
		OtelEndpoint: v.GetString("OTEL_EXPORTER_OTLP_ENDPOINT"),

		S3Endpoint:       v.GetString("S3_ENDPOINT"),
		S3PublicEndpoint: v.GetString("S3_PUBLIC_ENDPOINT"),
		S3Region:         v.GetString("S3_REGION"),
		S3Bucket:         v.GetString("S3_BUCKET"),
		S3AccessKey:      v.GetString("S3_ACCESS_KEY_ID"),
		S3SecretKey:      v.GetString("S3_SECRET_ACCESS_KEY"),
		S3ForcePathStyle: v.GetBool("S3_FORCE_PATH_STYLE"),

		FFmpegBin:  v.GetString("FFMPEG_BIN"),
		FFprobeBin: v.GetString("FFPROBE_BIN"),
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("APP_ENV", EnvDev)
	v.SetDefault("APP_PORT", "8083")
	v.SetDefault("LOG_LEVEL", "debug")
	v.SetDefault("TRUSTED_PROXIES", "")

	v.SetDefault("POSTGRES_HOST", "localhost")
	v.SetDefault("POSTGRES_PORT", "5433")
	v.SetDefault("POSTGRES_USER", "poro")
	v.SetDefault("POSTGRES_PASSWORD", "poro_dev_password")
	v.SetDefault("POSTGRES_DB", "poro_video")
	v.SetDefault("POSTGRES_SSLMODE", "disable")

	v.SetDefault("REDIS_HOST", "localhost")
	v.SetDefault("REDIS_PORT", "6380")
	v.SetDefault("REDIS_PASSWORD", "")
	v.SetDefault("REDIS_DB", 2)

	v.SetDefault("JWKS_URL", "http://localhost:8081/.well-known/jwks.json")
	v.SetDefault("KAFKA_BROKERS", "localhost:9092")
	v.SetDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	v.SetDefault("S3_ENDPOINT", "http://localhost:9000")
	v.SetDefault("S3_PUBLIC_ENDPOINT", "http://localhost:9000")
	v.SetDefault("S3_REGION", "us-east-1")
	v.SetDefault("S3_BUCKET", "poro-videos")
	v.SetDefault("S3_ACCESS_KEY_ID", "poro_dev")
	v.SetDefault("S3_SECRET_ACCESS_KEY", "poro_dev_secret")
	v.SetDefault("S3_FORCE_PATH_STYLE", true)

	v.SetDefault("FFMPEG_BIN", "ffmpeg")
	v.SetDefault("FFPROBE_BIN", "ffprobe")
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
