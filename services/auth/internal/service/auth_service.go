package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
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

// passwordParams follow the OWASP Password Storage minimum for Argon2id
// (19 MiB, 2 iterations, 1 lane). Stored hashes embed their own parameters,
// so older hashes stay verifiable.
var passwordParams = &argon2id.Params{
	Memory:      19 * 1024,
	Iterations:  2,
	Parallelism: 1,
	SaltLength:  16,
	KeyLength:   32,
}

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
	// ErrRefreshTokenReused is returned when a rotated token is replayed. The session is revoked.
	ErrRefreshTokenReused = errors.New("refresh token reused")
)

// AuthService orchestrates sign-in, token rotation, and logout.
type AuthService interface {
	RequestOTP(ctx context.Context, req dto.RequestOTPRequest) (*dto.RequestOTPResponse, error)
	VerifyOTP(ctx context.Context, req dto.VerifyOTPRequest, userAgent, ip string) (*dto.AuthResponse, error)
	RegisterEmail(ctx context.Context, req dto.RegisterEmailRequest, userAgent, ip string) (*dto.AuthResponse, error)
	LoginEmail(ctx context.Context, req dto.LoginEmailRequest, userAgent, ip string) (*dto.AuthResponse, error)
	Refresh(ctx context.Context, req dto.RefreshRequest) (*dto.RefreshResponse, error)
	Logout(ctx context.Context, claims TokenClaims) error
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
	reuseGrace time.Duration
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
	var reuseGrace time.Duration
	if cfg.JWTRefreshReuseGrace != "" {
		if reuseGrace, err = time.ParseDuration(cfg.JWTRefreshReuseGrace); err != nil || reuseGrace < 0 {
			return nil, fmt.Errorf("parse refresh reuse grace %q: must be a non-negative duration", cfg.JWTRefreshReuseGrace)
		}
	}
	return &authService{
		users:      users,
		refresh:    refresh,
		otp:        otp,
		tokens:     tokens,
		log:        log,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		reuseGrace: reuseGrace,
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
		user = &model.User{Phone: &phone, Roles: []model.UserRole{model.RolePersonal}, Status: model.StatusActive}
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
	hash, err := argon2id.CreateHash(req.Password, passwordParams)
	if err != nil {
		return nil, fmt.Errorf("register email: hash password: %w", err)
	}
	email := NormalizeEmail(req.Email)
	user := &model.User{
		Email:        &email,
		PasswordHash: &hash,
		Roles:        []model.UserRole{model.RolePersonal},
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
	user, err := s.users.GetByEmail(ctx, NormalizeEmail(req.Email))
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			burnPasswordCheck(req.Password)
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("login email: %w", err)
	}
	if user.PasswordHash == nil {
		burnPasswordCheck(req.Password)
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

// Refresh rotates a refresh token inside one transaction.
//
// A token replayed after rotation revokes its whole session family, except
// within the reuse grace window: a mobile client whose response was lost on a
// flaky network may retry once, which supersedes the unseen successor.
func (s *authService) Refresh(ctx context.Context, req dto.RefreshRequest) (*dto.RefreshResponse, error) {
	raw, nextHash, err := s.tokens.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("refresh token: %w", err)
	}
	nextID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("refresh token: new id: %w", err)
	}

	var res *dto.RefreshResponse
	var reused *model.RefreshToken
	var userID uuid.UUID
	err = s.refresh.InTx(ctx, func(tx repository.RefreshTokenTx) error {
		stored, err := tx.GetByHashForUpdate(ctx, s.tokens.HashToken(req.RefreshToken))
		if err != nil {
			if errors.Is(err, repository.ErrRefreshTokenNotFound) {
				return ErrRefreshTokenInvalid
			}
			return err
		}
		now := time.Now().UTC()
		if now.After(stored.ExpiresAt) {
			return ErrRefreshTokenInvalid
		}
		if stored.RevokedAt != nil {
			retry, err := s.supersedeWithinGrace(ctx, tx, stored, now)
			if err != nil {
				return err
			}
			if !retry {
				revoked, err := tx.RevokeFamily(ctx, stored.FamilyID, now)
				if err != nil {
					return err
				}
				if revoked == 0 {
					return ErrRefreshTokenInvalid
				}
				reused = stored
				return nil
			}
		}

		user, err := s.users.GetByID(ctx, stored.UserID)
		if err != nil {
			if errors.Is(err, repository.ErrUserNotFound) {
				return ErrRefreshTokenInvalid
			}
			return err
		}
		if blocked(user.Status) {
			return ErrAccountBlocked
		}

		next := &model.RefreshToken{
			ID:        nextID,
			UserID:    user.ID,
			FamilyID:  stored.FamilyID,
			TokenHash: nextHash,
			ExpiresAt: now.Add(s.refreshTTL),
			UserAgent: stored.UserAgent,
			IPAddress: stored.IPAddress,
			CreatedAt: now,
		}
		if err := tx.Create(ctx, next); err != nil {
			return err
		}
		if err := tx.MarkReplaced(ctx, stored.ID, next.ID, now); err != nil {
			return err
		}
		access, err := s.tokens.GenerateAccessToken(user.ID, user.RoleNames(), stored.FamilyID)
		if err != nil {
			return err
		}
		userID = user.ID
		res = &dto.RefreshResponse{
			AccessToken:  access,
			RefreshToken: raw,
			ExpiresIn:    int(s.accessTTL.Seconds()),
			TokenType:    tokenTypeBearer,
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("refresh token: %w", err)
	}
	if reused != nil {
		s.log.Warn("refresh token reuse detected, session revoked",
			zap.String("user_id", reused.UserID.String()),
			zap.String("session_id", reused.FamilyID.String()),
		)
		return nil, ErrRefreshTokenReused
	}
	s.log.Info("refresh token rotated", zap.String("user_id", userID.String()))
	return res, nil
}

// supersedeWithinGrace revokes the still-unused successor of a token rotated
// less than reuseGrace ago, and reports whether the caller may rotate again.
func (s *authService) supersedeWithinGrace(ctx context.Context, tx repository.RefreshTokenTx, stored *model.RefreshToken, now time.Time) (bool, error) {
	if stored.ReplacedBy == nil || now.Sub(*stored.RevokedAt) >= s.reuseGrace {
		return false, nil
	}
	successor, err := tx.GetByIDForUpdate(ctx, *stored.ReplacedBy)
	if err != nil {
		if errors.Is(err, repository.ErrRefreshTokenNotFound) {
			return false, nil
		}
		return false, err
	}
	if successor.RevokedAt != nil {
		return false, nil
	}
	if err := tx.Revoke(ctx, successor.ID, now); err != nil {
		return false, err
	}
	return true, nil
}

// Logout revokes the access token and every refresh token of its device session.
func (s *authService) Logout(ctx context.Context, claims TokenClaims) error {
	if err := s.tokens.BlacklistToken(ctx, claims.TokenID, time.Until(claims.Exp)); err != nil {
		return fmt.Errorf("logout: %w", err)
	}
	if _, err := s.refresh.RevokeFamily(ctx, claims.UserID, claims.SessionID); err != nil {
		return fmt.Errorf("logout: %w", err)
	}
	s.log.Info("logout succeeded",
		zap.String("user_id", claims.UserID.String()),
		zap.String("session_id", claims.SessionID.String()),
	)
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

// issueSession starts a new device session: a refresh token family and its first access token.
func (s *authService) issueSession(ctx context.Context, user *model.User, userAgent, ip string) (*dto.AuthResponse, error) {
	sessionID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("issue session: new id: %w", err)
	}
	access, err := s.tokens.GenerateAccessToken(user.ID, user.RoleNames(), sessionID)
	if err != nil {
		return nil, fmt.Errorf("issue session: %w", err)
	}
	raw, hash, err := s.tokens.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("issue session: %w", err)
	}
	token := &model.RefreshToken{
		ID:        sessionID,
		UserID:    user.ID,
		FamilyID:  sessionID,
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

// NormalizeEmail lowercases and trims an address. The database rejects any other form.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

var (
	dummyHashOnce sync.Once
	dummyHash     string
)

// burnPasswordCheck spends the same time as a real comparison, so response
// latency does not reveal whether an email is registered.
func burnPasswordCheck(password string) {
	dummyHashOnce.Do(func() {
		dummyHash, _ = argon2id.CreateHash("poro-timing-equalizer", passwordParams)
	})
	if dummyHash != "" {
		_, _ = argon2id.ComparePasswordAndHash(password, dummyHash)
	}
}

func toUserResponse(user *model.User) dto.UserResponse {
	return dto.UserResponse{
		ID:          user.ID.String(),
		Phone:       user.Phone,
		Email:       user.Email,
		Roles:       user.RoleNames(),
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
