package logging

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

const jwt = "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.c2lnbmF0dXJlLXZhbHVl"

func TestScrub(t *testing.T) {
	cases := map[string]string{
		"token " + jwt + " rejected":                    "token [REDACTED] rejected",
		"Authorization: Bearer abcdefghijklmnop":        "Authorization: Bearer [REDACTED]",
		"smtp: 550 awa.kone@example.ci unknown":         "smtp: 550 [REDACTED] unknown",
		"africastalking: +2250701020304 InvalidPhone":   "africastalking: [REDACTED] InvalidPhone",
		"video 0190f2a4-7c1e-7b3a-9d2e-1f2a3b4c5d6e ok": "video 0190f2a4-7c1e-7b3a-9d2e-1f2a3b4c5d6e ok",
		"phone +225*****04 masked by the service":       "phone +225*****04 masked by the service",
		"": "",
	}
	for in, want := range cases {
		require.Equal(t, want, Scrub(in), in)
	}
}

func TestRedactCore(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	log := zap.New(Redact(core)).With(zap.String("authorization", "Bearer abc"), zap.String("user_id", "u-1"))

	log.Info("login for awa@example.ci",
		zap.String("password", "hunter2"),
		zap.String("fcm_token", "device-token"),
		zap.String("request.otp", "123456"),
		zap.String("code", "invalid_media"),
		zap.Error(errors.New("upstream said "+jwt)),
		zap.Stringer("dest", stringer("+2250701020304")),
		zap.Int("attempt", 2),
	)

	entries := logs.All()
	require.Len(t, entries, 1)
	e := entries[0]
	require.Equal(t, "login for [REDACTED]", e.Message)
	fields := e.ContextMap()
	require.Equal(t, Redacted, fields["authorization"], "fields added with With are scrubbed too")
	require.Equal(t, "u-1", fields["user_id"])
	require.Equal(t, Redacted, fields["password"])
	require.Equal(t, Redacted, fields["fcm_token"])
	require.Equal(t, Redacted, fields["request.otp"], "the last segment of a key counts")
	require.Equal(t, "invalid_media", fields["code"], "failure codes stay readable")
	require.Equal(t, "upstream said [REDACTED]", fields["error"])
	require.Equal(t, Redacted, fields["dest"])
	require.EqualValues(t, 2, fields["attempt"])
}

func TestNewRedacts(t *testing.T) {
	log, err := New("auth", "info", false)
	require.NoError(t, err)
	_, ok := log.Core().(redactCore)
	require.True(t, ok, "every service logger goes through Redact")
}

type stringer string

func (s stringer) String() string { return string(s) }
