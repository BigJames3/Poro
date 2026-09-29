// Package dto defines the HTTP request and response bodies of the auth API.
package dto

// RequestOTPRequest is the body of POST /api/v1/auth/otp/request.
type RequestOTPRequest struct {
	Phone string `json:"phone" validate:"required,e164"`
}

// VerifyOTPRequest is the body of POST /api/v1/auth/otp/verify.
type VerifyOTPRequest struct {
	Phone string `json:"phone" validate:"required,e164"`
	Code  string `json:"code" validate:"required,len=6,numeric"`
}

// RegisterEmailRequest is the body of POST /api/v1/auth/email/register.
// FullName is validated and not stored: the profile belongs to the User service.
type RegisterEmailRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=72"`
	FullName string `json:"full_name" validate:"required,min=2,max=100"`
}

// LoginEmailRequest is the body of POST /api/v1/auth/email/login.
type LoginEmailRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// RefreshRequest is the body of POST /api/v1/auth/refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// LogoutRequest is the body of POST /api/v1/auth/logout.
// The access token is taken from the Authorization header.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// AuthResponse is returned after a successful sign-in or sign-up.
type AuthResponse struct {
	User         UserResponse `json:"user"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresIn    int          `json:"expires_in"`
	TokenType    string       `json:"token_type"`
}

// UserResponse is the public view of an auth account.
type UserResponse struct {
	ID          string  `json:"id"`
	Phone       *string `json:"phone,omitempty"`
	Email       *string `json:"email,omitempty"`
	Role        string  `json:"role"`
	Status      string  `json:"status"`
	CountryCode *string `json:"country_code,omitempty"`
	Language    string  `json:"language"`
	CreatedAt   string  `json:"created_at"`
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
}
