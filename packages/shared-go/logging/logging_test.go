package logging

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

func TestNew(t *testing.T) {
	log, err := New("user", "warn", false)
	require.NoError(t, err)
	require.False(t, log.Core().Enabled(zapcore.InfoLevel))
	require.True(t, log.Core().Enabled(zapcore.WarnLevel))

	dev, err := New("user", "debug", true)
	require.NoError(t, err)
	require.True(t, dev.Core().Enabled(zapcore.DebugLevel))

	_, err = New("user", "loud", false)
	require.Error(t, err)
}
