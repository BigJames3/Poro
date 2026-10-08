package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	ok := func() *Config {
		return &Config{
			AppEnv:           EnvDev,
			JWKSURL:          "http://localhost:8081/.well-known/jwks.json",
			KafkaBrokers:     []string{"localhost:9092"},
			S3Endpoint:       "http://localhost:9000",
			S3PublicEndpoint: "http://localhost:9000",
			S3Bucket:         "poro-videos",
			PostgresSSLMode:  "disable",

			S3QuarantineBucket: "poro-quarantine",
		}
	}
	require.NoError(t, ok().Validate())

	prod := ok()
	prod.AppEnv = EnvProd
	prod.PostgresSSLMode = "require"
	prod.S3AccessKey = "k"
	prod.S3SecretKey = "s"
	require.NoError(t, prod.Validate())

	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{name: "unknown env", mutate: func(c *Config) { c.AppEnv = "production" }, want: "APP_ENV"},
		{name: "missing jwks", mutate: func(c *Config) { c.JWKSURL = "" }, want: "JWKS_URL"},
		{name: "missing kafka", mutate: func(c *Config) { c.KafkaBrokers = nil }, want: "KAFKA_BROKERS"},
		{name: "missing bucket", mutate: func(c *Config) { c.S3Bucket = "" }, want: "S3_BUCKET"},
		{name: "missing quarantine", mutate: func(c *Config) { c.S3QuarantineBucket = "" }, want: "S3_QUARANTINE_BUCKET"},
		{name: "public quarantine", mutate: func(c *Config) { c.S3QuarantineBucket = "poro-videos" }, want: "must differ"},
		{name: "prod ssl", mutate: func(c *Config) {
			c.AppEnv = EnvProd
			c.PostgresSSLMode = "disable"
			c.S3AccessKey = "k"
			c.S3SecretKey = "s"
		}, want: "POSTGRES_SSLMODE"},
		{name: "prod creds", mutate: func(c *Config) { c.AppEnv = EnvProd; c.PostgresSSLMode = "require" }, want: "S3 credentials"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := ok()
			tc.mutate(c)
			err := c.Validate()
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestPublicObjectURL(t *testing.T) {
	c := &Config{S3PublicEndpoint: "http://localhost:9000/", S3Bucket: "poro-videos"}
	require.Equal(t, "http://localhost:9000/poro-videos/videos/a/hls/master.m3u8", c.PublicObjectURL("videos/a/hls/master.m3u8"))
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("JWKS_URL", "http://auth/.well-known/jwks.json")
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "8083", cfg.AppPort)
	require.Equal(t, "poro_video", cfg.PostgresDB)
	_ = os.Unsetenv("APP_ENV")
}
