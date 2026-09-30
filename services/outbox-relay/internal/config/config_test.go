package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	t.Setenv("RELAY_SOURCE", "auth")
	t.Setenv("POSTGRES_DB", "poro_auth")
	t.Setenv("POSTGRES_PASSWORD", "p@ss/word")
	t.Setenv("KAFKA_BROKERS", " redpanda:29092, ,other:9092 ")

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, []string{"redpanda:29092", "other:9092"}, cfg.KafkaBrokers)
	require.Equal(t, 500*time.Millisecond, cfg.PollInterval)
	require.Equal(t, "postgres://poro:p%40ss%2Fword@localhost:5433/poro_auth?sslmode=disable", cfg.PostgresDSN())

	t.Setenv("APP_ENV", "prod")
	_, err = Load()
	require.ErrorContains(t, err, "POSTGRES_SSLMODE")

	t.Setenv("APP_ENV", "dev")
	t.Setenv("RELAY_POLL_INTERVAL", "soon")
	_, err = Load()
	require.ErrorContains(t, err, "RELAY_POLL_INTERVAL")
	t.Setenv("RELAY_POLL_INTERVAL", "1s")
	t.Setenv("RELAY_RETENTION", "forever")
	_, err = Load()
	require.ErrorContains(t, err, "RELAY_RETENTION")
}

func TestValidate(t *testing.T) {
	err := (&Config{AppEnv: "qa", PostgresSSLMode: "disable"}).Validate()
	for _, want := range []string{"APP_ENV", "RELAY_SOURCE", "POSTGRES_DB", "KAFKA_BROKERS", "RELAY_BATCH_SIZE", "RELAY_POLL_INTERVAL", "RELAY_RETENTION"} {
		require.ErrorContains(t, err, want)
	}
	ok := &Config{AppEnv: "prod", Source: "user", PostgresDB: "poro_user", PostgresSSLMode: "verify-full",
		KafkaBrokers: []string{"k:9092"}, BatchSize: 100, PollInterval: time.Second, Retention: time.Hour}
	require.NoError(t, ok.Validate())
}
