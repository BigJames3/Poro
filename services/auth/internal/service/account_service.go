package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/poro/auth/internal/dto"
	"github.com/poro/auth/internal/model"
	"github.com/poro/auth/internal/repository"
)

var (
	// ErrSessionNotFound is returned when the session is not a live session of the caller.
	ErrSessionNotFound = errors.New("session not found")
	// ErrReauthRequired is returned when a sensitive action lacks the proof its account needs.
	ErrReauthRequired = errors.New("re-authentication required")
)

// maxDeviceLabelLen bounds the device label built from a User-Agent.
const maxDeviceLabelLen = 100

// AccountService manages the caller's device sessions and the deletion of their account.
type AccountService interface {
	Sessions(ctx context.Context, claims TokenClaims) ([]dto.SessionResponse, error)
	RevokeSession(ctx context.Context, claims TokenClaims, sessionID uuid.UUID) error
	RevokeOtherSessions(ctx context.Context, claims TokenClaims) (*dto.RevokeSessionsResponse, error)
	DeleteAccount(ctx context.Context, claims TokenClaims, req dto.DeleteAccountRequest) (*dto.DeletionResponse, error)
	CancelDeletion(ctx context.Context, claims TokenClaims) (*dto.UserResponse, error)
	SessionRevoker
}

// SessionRevoker signs every device of an account out, in every service.
type SessionRevoker interface {
	RevokeAllSessions(ctx context.Context, userID uuid.UUID, extra ...uuid.UUID) error
}

type accountService struct {
	users   repository.UserRepository
	refresh repository.RefreshTokenRepository
	otp     OTPService
	tokens  TokenService
	log     *zap.Logger
	grace   time.Duration
	now     func() time.Time
}

// NewAccountService wires the account operations. grace is the delay between
// a deletion request and the erasure of the account.
func NewAccountService(users repository.UserRepository, refresh repository.RefreshTokenRepository, otp OTPService, tokens TokenService, grace time.Duration, log *zap.Logger) (AccountService, error) {
	if grace <= 0 {
		return nil, fmt.Errorf("account deletion grace must be positive, got %s", grace)
	}
	return &accountService{
		users:   users,
		refresh: refresh,
		otp:     otp,
		tokens:  tokens,
		log:     log,
		grace:   grace,
		now:     func() time.Time { return time.Now().UTC() },
	}, nil
}

func (s *accountService) Sessions(ctx context.Context, claims TokenClaims) ([]dto.SessionResponse, error) {
	sessions, err := s.refresh.ActiveSessions(ctx, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("sessions: %w", err)
	}
	out := make([]dto.SessionResponse, len(sessions))
	for i, session := range sessions {
		out[i] = dto.SessionResponse{
			ID:           session.ID.String(),
			Device:       deviceLabel(session),
			IPAddress:    session.IPAddress,
			CreatedAt:    session.CreatedAt.UTC().Format(time.RFC3339),
			LastActiveAt: session.LastActiveAt.UTC().Format(time.RFC3339),
			Current:      session.ID == claims.SessionID,
		}
	}
	return out, nil
}

// RevokeSession signs a device out. Revoking the current session is a logout.
// The session enters the shared registry before its refresh tokens are
// revoked: if the registry fails nothing changed and the call can be retried,
// and if the database fails the session's access tokens are still refused.
func (s *accountService) RevokeSession(ctx context.Context, claims TokenClaims, sessionID uuid.UUID) error {
	if sessionID != claims.SessionID {
		owned, err := s.ownsSession(ctx, claims.UserID, sessionID)
		if err != nil {
			return fmt.Errorf("revoke session: %w", err)
		}
		if !owned {
			return ErrSessionNotFound
		}
	}
	if err := s.tokens.RevokeSessions(ctx, sessionID); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	if _, err := s.refresh.RevokeFamily(ctx, claims.UserID, sessionID); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	s.log.Info("session revoked",
		zap.String("user_id", claims.UserID.String()),
		zap.String("session_id", sessionID.String()),
	)
	return nil
}

func (s *accountService) RevokeOtherSessions(ctx context.Context, claims TokenClaims) (*dto.RevokeSessionsResponse, error) {
	sessions, err := s.refresh.ActiveSessions(ctx, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("revoke other sessions: %w", err)
	}
	others := make([]uuid.UUID, 0, len(sessions))
	for _, session := range sessions {
		if session.ID != claims.SessionID {
			others = append(others, session.ID)
		}
	}
	if err := s.tokens.RevokeSessions(ctx, others...); err != nil {
		return nil, fmt.Errorf("revoke other sessions: %w", err)
	}
	revoked, err := s.refresh.RevokeOtherFamilies(ctx, claims.UserID, claims.SessionID)
	if err != nil {
		return nil, fmt.Errorf("revoke other sessions: %w", err)
	}
	s.log.Info("other sessions revoked",
		zap.String("user_id", claims.UserID.String()),
		zap.Int64("revoked", revoked),
	)
	return &dto.RevokeSessionsResponse{Revoked: revoked}, nil
}

// RevokeAllSessions signs every device of userID out, everywhere: each live
// session enters the shared registry, plus extra ones (the caller's session,
// which may have no live refresh token left), then the refresh tokens are
// revoked.
func (s *accountService) RevokeAllSessions(ctx context.Context, userID uuid.UUID, extra ...uuid.UUID) error {
	sessions, err := s.refresh.ActiveSessions(ctx, userID)
	if err != nil {
		return fmt.Errorf("revoke all sessions: %w", err)
	}
	ids := append([]uuid.UUID(nil), extra...)
	for _, session := range sessions {
		ids = append(ids, session.ID)
	}
	if err := s.tokens.RevokeSessions(ctx, ids...); err != nil {
		return fmt.Errorf("revoke all sessions: %w", err)
	}
	if err := s.refresh.RevokeAllForUser(ctx, userID); err != nil {
		return fmt.Errorf("revoke all sessions: %w", err)
	}
	return nil
}

func (s *accountService) ownsSession(ctx context.Context, userID, sessionID uuid.UUID) (bool, error) {
	sessions, err := s.refresh.ActiveSessions(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, session := range sessions {
		if session.ID == sessionID {
			return true, nil
		}
	}
	return false, nil
}

// DeleteAccount schedules the erasure of the caller's account after the
// owner proved it again, then signs every device out. Asking twice keeps the
// first date.
func (s *accountService) DeleteAccount(ctx context.Context, claims TokenClaims, req dto.DeleteAccountRequest) (*dto.DeletionResponse, error) {
	user, err := s.users.GetByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("delete account: %w", err)
	}
	if blocked(user.Status) {
		return nil, ErrAccountBlocked
	}
	if err := s.reauthenticate(ctx, user, req); err != nil {
		return nil, fmt.Errorf("delete account: %w", err)
	}
	scheduled, err := s.users.ScheduleDeletion(ctx, user.ID, s.now().Add(s.grace))
	if err != nil {
		return nil, fmt.Errorf("delete account: %w", err)
	}
	if err := s.RevokeAllSessions(ctx, user.ID, claims.SessionID); err != nil {
		return nil, fmt.Errorf("delete account: %w", err)
	}
	s.log.Info("account deletion scheduled",
		zap.String("user_id", user.ID.String()),
		zap.Time("deletion_scheduled_at", scheduled),
	)
	return &dto.DeletionResponse{DeletionScheduledAt: scheduled.UTC().Format(time.RFC3339)}, nil
}

func (s *accountService) CancelDeletion(ctx context.Context, claims TokenClaims) (*dto.UserResponse, error) {
	cancelled, err := s.users.CancelDeletion(ctx, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("cancel deletion: %w", err)
	}
	user, err := s.users.GetByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("cancel deletion: %w", err)
	}
	if cancelled {
		s.log.Info("account deletion cancelled", zap.String("user_id", user.ID.String()))
	}
	res := toUserResponse(user)
	return &res, nil
}

// reauthenticate asks for the password of an email account, or a fresh OTP
// code for an account that signs in by phone only.
func (s *accountService) reauthenticate(ctx context.Context, user *model.User, req dto.DeleteAccountRequest) error {
	switch {
	case user.PasswordHash != nil:
		if req.Password == "" {
			return ErrReauthRequired
		}
		match, err := argon2id.ComparePasswordAndHash(req.Password, *user.PasswordHash)
		if err != nil {
			return fmt.Errorf("compare password: %w", err)
		}
		if !match {
			return ErrInvalidCredentials
		}
		return nil
	case user.Phone != nil:
		if req.OTPCode == "" {
			return ErrReauthRequired
		}
		_, err := s.otp.VerifyOTP(ctx, *user.Phone, req.OTPCode)
		return err
	default:
		return ErrReauthRequired
	}
}

// deviceLabel names a session: the name the app gave, else its User-Agent.
func deviceLabel(session model.Session) *string {
	if session.DeviceName != nil && *session.DeviceName != "" {
		return session.DeviceName
	}
	if session.UserAgent != nil && *session.UserAgent != "" {
		label := truncateRunes(*session.UserAgent, maxDeviceLabelLen)
		return &label
	}
	return nil
}
