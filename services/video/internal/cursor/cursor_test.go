package cursor

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecode(t *testing.T) {
	id := uuid.Must(uuid.NewV7())
	ts := time.Date(2026, 9, 30, 12, 0, 0, 123, time.UTC)
	gotTs, gotID, err := Decode(Encode(ts, id))
	require.NoError(t, err)
	require.True(t, ts.Equal(gotTs))
	require.Equal(t, id, gotID)

	_, _, err = Decode("%%%")
	require.Error(t, err)
	_, _, err = Decode(Encode(ts, id)[:4])
	require.Error(t, err)
}
