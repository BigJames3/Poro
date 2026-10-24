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

	"github.com/poro/shared-go/revocation"
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

	JWKSURL string
	// RevocationRedisURL is the registry of revoked sessions written by auth
	// (ADR-0016). Required outside dev.
	RevocationRedisURL string
	KafkaBrokers       []string
	OtelEndpoint       string

	S3Endpoint       string
	S3PublicEndpoint string
	S3Region         string
	// S3Bucket is the publicly readable media bucket: HLS renditions and
	// thumbnails only.
	S3Bucket string
	// S3SourceBucket holds the original uploads. It must not be publicly
	// readable.
	S3SourceBucket string
	// S3QuarantineBucket holds the media of removed and deleted videos. It
	// must not be publicly readable.
	S3QuarantineBucket string
	S3AccessKey        string
	S3SecretKey        string
	S3ForcePathStyle   bool

	// PhotoPostsEnabled opens the photo post upload routes (ADR-0020). Off by
	// default: it stays off until the feed, search, social and notification
	// services keep photo posts away from clients without the photo-posts
	// capability (step 2d). Video routes never depend on it.
	PhotoPostsEnabled bool

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
	errs := []error{c.ValidateStorage()}
	if c.PostgresPassword == "" {
		errs = append(errs, errors.New("POSTGRES_PASSWORD is required"))
	}
	if c.JWKSURL == "" {
		errs = append(errs, errors.New("JWKS_URL is required"))
	}
	if err := revocation.CheckURL(c.RevocationRedisURL, c.IsDev()); err != nil {
		errs = append(errs, err)
	}
	if len(c.KafkaBrokers) == 0 {
		errs = append(errs, errors.New("KAFKA_BROKERS is required"))
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

// ValidateStorage checks only what object storage needs. Commands that touch
// nothing but buckets, such as storage-migrate, use it so that they are never
// handed database or broker credentials they do not use.
func (c *Config) ValidateStorage() error {
	errs := []error{c.validateAppEnv()}
	if c.S3Endpoint == "" || c.S3Bucket == "" {
		errs = append(errs, errors.New("S3_ENDPOINT and S3_BUCKET are required"))
	}
	if c.S3QuarantineBucket == "" || c.S3QuarantineBucket == c.S3Bucket {
		errs = append(errs, errors.New("S3_QUARANTINE_BUCKET is required and must differ from S3_BUCKET"))
	}
	if c.S3SourceBucket == "" || c.S3SourceBucket == c.S3Bucket {
		errs = append(errs, errors.New("S3_SOURCE_BUCKET is required and must differ from S3_BUCKET"))
	}
	if c.S3PublicEndpoint == "" {
		errs = append(errs, errors.New("S3_PUBLIC_ENDPOINT is required"))
	}
	if c.S3AccessKey == "" || c.S3SecretKey == "" {
		errs = append(errs, errors.New("S3_ACCESS_KEY_ID and S3_SECRET_ACCESS_KEY are required"))
	}
	return errors.Join(errs...)
}

func (c *Config) validateAppEnv() error {
	switch c.AppEnv {
	case EnvDev, EnvStaging, EnvProd:
	case "":
		// No default: a forgotten APP_ENV must not start a production
		// service with the shortcuts of local development.
		return errors.New("APP_ENV is required: dev, staging or prod")
	default:
		return fmt.Errorf("APP_ENV must be dev, staging or prod, got %q", c.AppEnv)
	}
	return nil
}

// Load reads configuration from the environment and an optional .env file.
func Load() (*Config, error) {
	cfg, err := read()
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return cfg, nil
}

// LoadStorage reads the same sources as Load but only requires the object
// storage settings (see ValidateStorage).
func LoadStorage() (*Config, error) {
	cfg, err := read()
	if err != nil {
		return nil, err
	}
	if err := cfg.ValidateStorage(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return cfg, nil
}

func read() (*Config, error) {
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

		JWKSURL:            v.GetString("JWKS_URL"),
		RevocationRedisURL: v.GetString("REVOCATION_REDIS_URL"),
		KafkaBrokers:       splitList(v.GetString("KAFKA_BROKERS")),
		OtelEndpoint:       v.GetString("OTEL_EXPORTER_OTLP_ENDPOINT"),

		S3Endpoint:         v.GetString("S3_ENDPOINT"),
		S3PublicEndpoint:   v.GetString("S3_PUBLIC_ENDPOINT"),
		S3Region:           v.GetString("S3_REGION"),
		S3Bucket:           v.GetString("S3_BUCKET"),
		S3SourceBucket:     v.GetString("S3_SOURCE_BUCKET"),
		S3QuarantineBucket: v.GetString("S3_QUARANTINE_BUCKET"),
		S3AccessKey:        v.GetString("S3_ACCESS_KEY_ID"),
		S3SecretKey:        v.GetString("S3_SECRET_ACCESS_KEY"),
		S3ForcePathStyle:   v.GetBool("S3_FORCE_PATH_STYLE"),

		PhotoPostsEnabled: v.GetBool("PHOTO_POSTS_ENABLED"),

		FFmpegBin:  v.GetString("FFMPEG_BIN"),
		FFprobeBin: v.GetString("FFPROBE_BIN"),
	}
	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("APP_PORT", "8083")
	v.SetDefault("PHOTO_POSTS_ENABLED", false)
	v.SetDefault("LOG_LEVEL", "debug")
	v.SetDefault("TRUSTED_PROXIES", "")

	v.SetDefault("POSTGRES_HOST", "localhost")
	v.SetDefault("POSTGRES_PORT", "5433")
	v.SetDefault("POSTGRES_USER", "poro")
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
	v.SetDefault("S3_SOURCE_BUCKET", "poro-video-sources")
	v.SetDefault("S3_QUARANTINE_BUCKET", "poro-quarantine")
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
