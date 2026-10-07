package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	ok := func() *Config {
		return &Config{
			AppEnv:          EnvDev,
			JWKSURL:         "http://localhost:8081/.well-known/jwks.json",
			KafkaBrokers:    []string{"localhost:9092"},
			PostgresSSLMode: "disable",
		}
	}
	require.NoError(t, ok().Validate())

	prod := ok()
	prod.AppEnv = EnvProd
	prod.PostgresSSLMode = "verify-full"
	require.NoError(t, prod.Validate())

	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{name: "unknown env", mutate: func(c *Config) { c.AppEnv = "production" }, want: "APP_ENV"},
		{name: "missing jwks", mutate: func(c *Config) { c.JWKSURL = "" }, want: "JWKS_URL"},
		{name: "missing kafka", mutate: func(c *Config) { c.KafkaBrokers = nil }, want: "KAFKA_BROKERS"},
		{name: "prod without tls", mutate: func(c *Config) { c.AppEnv = EnvProd }, want: "POSTGRES_SSLMODE"},
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

func TestPostgresDSNEscapesCredentials(t *testing.T) {
	c := &Config{
		PostgresUser: "poro", PostgresPassword: "p@ss:w/rd", PostgresHost: "db",
		PostgresPort: "5432", PostgresDB: "poro_social", PostgresSSLMode: "disable",
	}
	require.Equal(t, "postgres://poro:p%40ss%3Aw%2Frd@db:5432/poro_social?sslmode=disable", c.PostgresDSN())
}

func TestLoadDefaultsAndEnv(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("KAFKA_BROKERS", "a:9092, b:9092 ,")
	t.Setenv("REDIS_DB", "5")
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "8085", cfg.AppPort)
	require.Equal(t, "poro_social", cfg.PostgresDB)
	require.Equal(t, []string{"a:9092", "b:9092"}, cfg.KafkaBrokers)
	require.Equal(t, 5, cfg.RedisDB)
	require.True(t, cfg.IsDev())
}

func TestLoadReadsDotEnvAndRejectsInvalid(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte("APP_ENV=prod\n"), 0o600))
	_, err := Load()
	require.Error(t, err)
	require.Contains(t, err.Error(), "POSTGRES_SSLMODE")
}
