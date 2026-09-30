package model

import (
	"time"

	"github.com/google/uuid"
)

// RefreshToken is a stored refresh credential. TokenHash is the SHA-256 of the raw token.
// FamilyID identifies the device session: every rotation keeps it, so reuse of an
// old token revokes the whole session. ReplacedBy points to the successor token.
type RefreshToken struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	UserID     uuid.UUID  `json:"user_id" db:"user_id"`
	FamilyID   uuid.UUID  `json:"family_id" db:"family_id"`
	TokenHash  string     `json:"-" db:"token_hash"`
	ExpiresAt  time.Time  `json:"expires_at" db:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
	ReplacedBy *uuid.UUID `json:"replaced_by,omitempty" db:"replaced_by"`
	UserAgent  *string    `json:"user_agent,omitempty" db:"user_agent"`
	IPAddress  *string    `json:"ip_address,omitempty" db:"ip_address"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
}
