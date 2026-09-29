package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/poro/auth/internal/config"
	"github.com/poro/auth/internal/dto"
	"github.com/poro/auth/internal/model"
	"github.com/poro/auth/internal/repository"
)

const tokenTypeBearer = "Bearer"

var (
	// ErrEmailTaken is returned when the email is already registered.
	ErrEmailTaken = errors.New("email already registered")
	// ErrInvalidCredentials is returned when the email or password does not match.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrAccountBlocked is returned when the account is suspended or deleted.
	ErrAccountBlocked = errors.New("account blocked")
	// ErrAccountNotFound is returned when the current account does not exist.
	ErrAccountNotFound = errors.New("account not found")
	// ErrRefreshTokenInvalid is returned when a refresh token is unknown, expired, or revoked.
	ErrRefreshTokenInvalid = errors.New("invalid refresh token")
)

// AuthService orchestrates sign-in, token rotation, and logout.
type AuthService interface {
	RequestOTP(ctx context.Context, req dto.RequestOTPRequest) (*dto.RequestOTPResponse, error)
	VerifyOTP(ctx context.Context, req dto.VerifyOTPRequest, userAgent, ip string) (*dto.AuthResponse, error)
	RegisterEmail(ctx context.Context, req dto.RegisterEmailRequest, userAgent, ip string) (*dto.AuthResponse, error)
	LoginEmail(ctx context.Context, req dto.LoginEmailRequest, userAgent, ip string) (*dto.AuthResponse, error)
	Refresh(ctx context.Context, req dto.RefreshRequest) (*dto.RefreshResponse, error)
	Logout(ctx context.Context, accessToken, refreshToken string) error
	Me(ctx context.Context, userID uuid.UUID) (*dto.UserResponse, error)
}

type authService struct {
	users      repository.UserRepository
	refresh    repository.RefreshTokenRepository
	otp        OTPService
	tokens     TokenService
	log        *zap.Logger
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewAuthService wires the repositories and token services.
// FullName from email registration is ignored: profiles belong to the User service.
func NewAuthService(cfg *config.Config, users repository.UserRepository, refresh repository.RefreshTokenRepository, otp OTPService, tokens TokenService, log *zap.Logger) (AuthService, error) {
	accessTTL, err := time.ParseDuration(cfg.JWTAccessTTL)
	if err != nil {
		return nil, fmt.Errorf("parse access token ttl: %w", err)
	}
	refreshTTL, err := time.ParseDuration(cfg.JWTRefreshTTL)
	if err != nil {
		return nil, fmt.Errorf("parse refresh token ttl: %w", err)
	}
	if accessTTL <= 0 || refreshTTL <= 0 {
		return nil, fmt.Errorf("token ttl must be positive")
	}
	return &authService{
		users:      users,
		refresh:    refresh,
		otp:        otp,
		tokens:     tokens,
		log:        log,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}, nil
}

func (s *authService) RequestOTP(ctx context.Context, req dto.RequestOTPRequest) (*dto.RequestOTPResponse, error) {
	expiresIn, err := s.otp.RequestOTP(ctx, req.Phone)
	if err != nil {
		return nil, fmt.Errorf("request otp: %w", err)
	}
	return &dto.RequestOTPResponse{
		Message:   "otp sent",
		ExpiresIn: expiresIn,
	}, nil
}

func (s *authService) VerifyOTP(ctx context.Context, req dto.VerifyOTPRequest, userAgent, ip string) (*dto.AuthResponse, error) {
	if _, err := s.otp.VerifyOTP(ctx, req.Phone, req.Code); err != nil {
		return nil, fmt.Errorf("verify otp: %w", err)
	}

	user, err := s.users.GetByPhone(ctx, req.Phone)
	if err != nil && !errors.Is(err, repository.ErrUserNotFound) {
		return nil, fmt.Errorf("verify otp: %w", err)
	}
	if errors.Is(err, repository.ErrUserNotFound) {
		phone := req.Phone
		user = &model.User{Phone: &phone, Role: model.RolePersonal, Status: model.StatusActive}
		if err := s.users.Create(ctx, user); err != nil {
			if !errors.Is(err, repository.ErrUserAlreadyExists) {
				return nil, fmt.Errorf("verify otp: %w", err)
			}
			user, err = s.users.GetByPhone(ctx, req.Phone)
			if err != nil {
				return nil, fmt.Errorf("verify otp: %w", err)
			}
		}
	}
	if err := s.ensureActive(ctx, user); err != nil {
		return nil, fmt.Errorf("verify otp: %w", err)
	}

	res, err := s.issueSession(ctx, user, userAgent, ip)
	if err != nil {
		return nil, fmt.Errorf("verify otp: %w", err)
	}
	s.log.Info("otp login succeeded", zap.String("user_id", user.ID.String()))
	return res, nil
}

func (s *authService) RegisterEmail(ctx context.Context, req dto.RegisterEmailRequest, userAgent, ip string) (*dto.AuthResponse, error) {
	hash, err := argon2id.CreateHash(req.Password, argon2id.DefaultParams)
	if err != nil {
		return nil, fmt.Errorf("register email: hash password: %w", err)
	}
	email := req.Email
	user := &model.User{
		Email:        &email,
		PasswordHash: &hash,
		Role:         model.RolePersonal,
		Status:       model.StatusActive,
	}
	if err := s.users.Create(ctx, user); err != nil {
		if errors.Is(err, repository.ErrUserAlreadyExists) {
			return nil, fmt.Errorf("register email: %w", ErrEmailTaken)
		}
		return nil, fmt.Errorf("register email: %w", err)
	}

	res, err := s.issueSession(ctx, user, userAgent, ip)
	if err != nil {
		return nil, fmt.Errorf("register email: %w", err)
	}
	s.log.Info("email registration succeeded", zap.String("user_id", user.ID.String()))
	return res, nil
}

func (s *authService) LoginEmail(ctx context.Context, req dto.LoginEmailRequest, userAgent, ip string) (*dto.AuthResponse, error) {
	user, err := s.users.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("login email: %w", err)
	}
	if blocked(user.Status) {
		return nil, ErrAccountBlocked
	}
	if user.PasswordHash == nil {
		return nil, ErrInvalidCredentials
	}
	match, err := argon2id.ComparePasswordAndHash(req.Password, *user.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("login email: compare password: %w", err)
	}
	if !match {
		return nil, ErrInvalidCredentials
	}
	if err := s.ensureActive(ctx, user); err != nil {
		return nil, fmt.Errorf("login email: %w", err)
	}

	res, err := s.issueSession(ctx, user, userAgent, ip)
	if err != nil {
		return nil, fmt.Errorf("login email: %w", err)
	}
	s.log.Info("email login succeeded", zap.String("user_id", user.ID.String()))
	return res, nil
}

func (s *authService) Refresh(ctx context.Context, req dto.RefreshRequest) (*dto.RefreshResponse, error) {
	stored, err := s.refresh.GetByHash(ctx, s.tokens.HashToken(req.RefreshToken))
	if err != nil {
		if errors.Is(err, repository.ErrRefreshTokenNotFound) {
			return nil, ErrRefreshTokenInvalid
		}
		return nil, fmt.Errorf("refresh token: %w", err)
	}
	if stored.RevokedAt != nil || time.Now().UTC().After(stored.ExpiresAt) {
		return nil, ErrRefreshTokenInvalid
	}

	user, err := s.users.GetByID(ctx, stored.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrRefreshTokenInvalid
		}
		return nil, fmt.Errorf("refresh token: %w", err)
	}
	if blocked(user.Status) {
		return nil, ErrAccountBlocked
	}

	access, err := s.tokens.GenerateAccessToken(user.ID, string(user.Role))
	if err != nil {
		return nil, fmt.Errorf("refresh token: %w", err)
	}
	raw, hash, err := s.tokens.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("refresh token: %w", err)
	}
	next := &model.RefreshToken{
		UserID:    user.ID,
		TokenHash: hash,
		ExpiresAt: time.Now().UTC().Add(s.refreshTTL),
		UserAgent: stored.UserAgent,
		IPAddress: stored.IPAddress,
	}
	if err := s.refresh.Create(ctx, next); err != nil {
		return nil, fmt.Errorf("refresh token: %w", err)
	}
	if err := s.refresh.Revoke(ctx, stored.ID); err != nil {
		return nil, fmt.Errorf("refresh token: %w", err)
	}

	s.log.Info("refresh token rotated", zap.String("user_id", user.ID.String()))
	return &dto.RefreshResponse{
		AccessToken:  access,
		RefreshToken: raw,
		ExpiresIn:    int(s.accessTTL.Seconds()),
	}, nil
}

func (s *authService) Logout(ctx context.Context, accessToken, refreshToken string) error {
	claims, err := s.tokens.ValidateAccessToken(accessToken)
	if err != nil {
		return fmt.Errorf("logout: %w", err)
	}
	if err := s.tokens.BlacklistToken(ctx, accessToken, time.Until(claims.Exp)); err != nil {
		return fmt.Errorf("logout: %w", err)
	}
	if refreshToken != "" {
		stored, err := s.refresh.GetByHash(ctx, s.tokens.HashToken(refreshToken))
		if err != nil && !errors.Is(err, repository.ErrRefreshTokenNotFound) {
			return fmt.Errorf("logout: %w", err)
		}
		if err == nil && stored.RevokedAt == nil {
			if err := s.refresh.Revoke(ctx, stored.ID); err != nil {
				return fmt.Errorf("logout: %w", err)
			}
		}
	}
	s.log.Info("logout succeeded", zap.String("user_id", claims.UserID.String()))
	return nil
}

func (s *authService) Me(ctx context.Context, userID uuid.UUID) (*dto.UserResponse, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("me: %w", err)
	}
	res := toUserResponse(user)
	return &res, nil
}

func (s *authService) ensureActive(ctx context.Context, user *model.User) error {
	if blocked(user.Status) {
		return ErrAccountBlocked
	}
	if user.Status != model.StatusPending {
		return nil
	}
	user.Status = model.StatusActive
	if err := s.users.Update(ctx, user); err != nil {
		return fmt.Errorf("activate account: %w", err)
	}
	return nil
}

func (s *authService) issueSession(ctx context.Context, user *model.User, userAgent, ip string) (*dto.AuthResponse, error) {
	access, err := s.tokens.GenerateAccessToken(user.ID, string(user.Role))
	if err != nil {
		return nil, fmt.Errorf("issue session: %w", err)
	}
	raw, hash, err := s.tokens.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("issue session: %w", err)
	}
	token := &model.RefreshToken{
		UserID:    user.ID,
		TokenHash: hash,
		ExpiresAt: time.Now().UTC().Add(s.refreshTTL),
		UserAgent: optionalString(userAgent),
		IPAddress: optionalString(ip),
	}
	if err := s.refresh.Create(ctx, token); err != nil {
		return nil, fmt.Errorf("issue session: %w", err)
	}
	if err := s.users.UpdateLastLogin(ctx, user.ID); err != nil {
		return nil, fmt.Errorf("issue session: %w", err)
	}
	return &dto.AuthResponse{
		User:         toUserResponse(user),
		AccessToken:  access,
		RefreshToken: raw,
		ExpiresIn:    int(s.accessTTL.Seconds()),
		TokenType:    tokenTypeBearer,
	}, nil
}

func toUserResponse(user *model.User) dto.UserResponse {
	return dto.UserResponse{
		ID:          user.ID.String(),
		Phone:       user.Phone,
		Email:       user.Email,
		Role:        string(user.Role),
		Status:      string(user.Status),
		CountryCode: user.CountryCode,
		Language:    user.Language,
		CreatedAt:   user.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func blocked(status model.UserStatus) bool {
	return status == model.StatusSuspended || status == model.StatusDeleted
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
