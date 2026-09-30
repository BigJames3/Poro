package model

import (
	"time"

	"github.com/google/uuid"
)

// OTPCode is a one-time SMS code. CodeHash is the hex HMAC-SHA256 of the raw code,
// keyed with the server OTP secret.
type OTPCode struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	Phone       string     `json:"phone" db:"phone"`
	CodeHash    string     `json:"-" db:"code_hash"`
	Attempts    int        `json:"attempts" db:"attempts"`
	MaxAttempts int        `json:"max_attempts" db:"max_attempts"`
	ExpiresAt   time.Time  `json:"expires_at" db:"expires_at"`
	VerifiedAt  *time.Time `json:"verified_at,omitempty" db:"verified_at"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
}
