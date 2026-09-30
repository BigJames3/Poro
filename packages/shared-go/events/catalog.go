package events

import "time"

// Event types. The JSON Schema of each lives in packages/contracts/events.
const (
	TypeAuthUserCreated      = "poro.auth.user.created"
	TypeUserCreatorActivated = "poro.user.creator.activated"
)

// AuthUserCreatedV1 is published by auth when an account is created.
// It carries no phone number or email: consumers never need them.
type AuthUserCreatedV1 struct {
	UserID       string    `json:"user_id"`
	SignupMethod string    `json:"signup_method"` // phone, email
	CountryCode  *string   `json:"country_code"`
	Language     string    `json:"language"`
	CreatedAt    time.Time `json:"created_at"`
}

// UserCreatorActivatedV1 is published by the user service when a profile becomes a creator.
type UserCreatorActivatedV1 struct {
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	ActivatedAt time.Time `json:"activated_at"`
}
