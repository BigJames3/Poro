package model

import (
	"time"

	"github.com/google/uuid"
)

// Session is a signed-in device: a refresh token family seen through its live token.
type Session struct {
	ID           uuid.UUID // family id, the sid claim of its access tokens
	DeviceName   *string
	UserAgent    *string
	IPAddress    *string
	CreatedAt    time.Time // sign-in
	LastActiveAt time.Time // latest refresh
}
