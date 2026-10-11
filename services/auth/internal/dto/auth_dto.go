// Package dto defines the HTTP request and response bodies of the auth API.
package dto

// RequestOTPRequest is the body of POST /api/v1/auth/otp/request.
type RequestOTPRequest struct {
	Phone string `json:"phone" validate:"required,e164"`
}

// VerifyOTPRequest is the body of POST /api/v1/auth/otp/verify.
type VerifyOTPRequest struct {
	Phone      string `json:"phone" validate:"required,e164"`
	Code       string `json:"code" validate:"required,len=6,numeric"`
	DeviceName string `json:"device_name" validate:"max=60"`
}

// RegisterEmailRequest is the body of POST /api/v1/auth/email/register.
// FullName is validated and not stored: the profile belongs to the User service.
type RegisterEmailRequest struct {
	Email      string `json:"email" validate:"required,email,max=254"`
	Password   string `json:"password" validate:"required,min=8,max=128"`
	FullName   string `json:"full_name" validate:"required,min=2,max=100"`
	DeviceName string `json:"device_name" validate:"max=60"`
}

// LoginEmailRequest is the body of POST /api/v1/auth/email/login.
type LoginEmailRequest struct {
	Email      string `json:"email" validate:"required,email,max=254"`
	Password   string `json:"password" validate:"required,max=128"`
	DeviceName string `json:"device_name" validate:"max=60"`
}

// RefreshRequest is the body of POST /api/v1/auth/refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required,max=256"`
}

// AuthResponse is returned after a successful sign-in or sign-up.
type AuthResponse struct {
	User         UserResponse `json:"user"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresIn    int          `json:"expires_in"`
	TokenType    string       `json:"token_type"`
	// DeletionCancelled is true when this sign-in cancelled a scheduled deletion.
	DeletionCancelled bool `json:"deletion_cancelled,omitempty"`
}

// UserResponse is the public view of an auth account.
type UserResponse struct {
	ID          string   `json:"id"`
	Phone       *string  `json:"phone,omitempty"`
	Email       *string  `json:"email,omitempty"`
	Roles       []string `json:"roles"`
	Status      string   `json:"status"`
	CountryCode *string  `json:"country_code,omitempty"`
	Language    string   `json:"language"`
	CreatedAt   string   `json:"created_at"`
	// EmailVerified is true once the owner proved they read Email.
	EmailVerified bool `json:"email_verified"`
	// DeletionScheduledAt is when the account will be erased; null when no deletion is pending.
	DeletionScheduledAt *string `json:"deletion_scheduled_at"`
}

// RequestOTPResponse is returned after an OTP is issued.
type RequestOTPResponse struct {
	Message   string `json:"message"`
	ExpiresIn int    `json:"expires_in"`
}

// RefreshResponse is returned after a refresh token is rotated.
type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// DeleteAccountRequest is the body of POST /api/v1/auth/me/delete. The owner
// proves it again: password for an email account, a fresh OTP code (from
// POST /otp/request) for a phone account.
type DeleteAccountRequest struct {
	Password string `json:"password" validate:"max=128"`
	OTPCode  string `json:"otp_code" validate:"omitempty,len=6,numeric"`
}

// DeletionResponse is returned once the deletion is scheduled.
type DeletionResponse struct {
	DeletionScheduledAt string `json:"deletion_scheduled_at"`
}

// SessionResponse is a signed-in device of the account.
type SessionResponse struct {
	ID           string  `json:"id"`
	Device       *string `json:"device"`
	IPAddress    *string `json:"ip_address"`
	CreatedAt    string  `json:"created_at"`
	LastActiveAt string  `json:"last_active_at"`
	Current      bool    `json:"current"`
}

// RevokeSessionsResponse tells how many other devices were signed out.
type RevokeSessionsResponse struct {
	Revoked int64 `json:"revoked"`
}

// ConfirmEmailRequest is the body of POST /api/v1/auth/email/verification/confirm.
type ConfirmEmailRequest struct {
	Code string `json:"code" validate:"required,len=6,numeric"`
}

// CodeSentResponse is returned once a code is on its way.
type CodeSentResponse struct {
	ExpiresIn int `json:"expires_in"`
}

// ForgotPasswordRequest is the body of POST /api/v1/auth/email/password/forgot.
type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email,max=254"`
}

// ResetPasswordRequest is the body of POST /api/v1/auth/email/password/reset.
type ResetPasswordRequest struct {
	Email       string `json:"email" validate:"required,email,max=254"`
	Code        string `json:"code" validate:"required,len=6,numeric"`
	NewPassword string `json:"new_password" validate:"required,min=8,max=128"`
}
