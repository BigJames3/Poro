package model

import (
	"time"

	"github.com/google/uuid"
)

// EmailCodePurpose says what an email code unlocks.
type EmailCodePurpose string

const (
	// EmailCodeVerify proves the account owner reads its address.
	EmailCodeVerify EmailCodePurpose = "verify"
	// EmailCodeReset lets the owner choose a new password.
	EmailCodeReset EmailCodePurpose = "reset"
)

// EmailCode is a six-digit code sent by email. CodeHash is the hex HMAC-SHA256
// of the code, bound to the account, the address and the purpose.
type EmailCode struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Purpose     EmailCodePurpose
	Email       string
	CodeHash    string
	Attempts    int
	MaxAttempts int
	ExpiresAt   time.Time
	ConsumedAt  *time.Time
	CreatedAt   time.Time
}
