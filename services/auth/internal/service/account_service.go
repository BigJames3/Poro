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
	// ErrSuspensionForbidden is returned when the caller may not suspend or
	// reinstate this account: only ADMIN and MODERATOR may, and a MODERATOR
	// never acts on an ADMIN or another MODERATOR.
	ErrSuspensionForbidden = errors.New("not allowed to suspend this account")
	// ErrSelfSuspension is returned when an administrator targets their own account.
	ErrSelfSuspension = errors.New("an administrator cannot suspend their own account")
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
	Suspend(ctx context.Context, admin TokenClaims, userID uuid.UUID) (*dto.UserResponse, error)
	Reinstate(ctx context.Context, admin TokenClaims, userID uuid.UUID) (*dto.UserResponse, error)
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
	logins  LoginGuard
	log     *zap.Logger
	grace   time.Duration
	now     func() time.Time
}

// NewAccountService wires the account operations. grace is the delay between
// a deletion request and the erasure of the account.
func NewAccountService(users repository.UserRepository, refresh repository.RefreshTokenRepository, otp OTPService, tokens TokenService, logins LoginGuard, grace time.Duration, log *zap.Logger) (AccountService, error) {
	if grace <= 0 {
		return nil, fmt.Errorf("account deletion grace must be positive, got %s", grace)
	}
	if logins == nil {
		return nil, errors.New("account service: login guard is required")
	}
	return &accountService{
		users:   users,
		refresh: refresh,
		otp:     otp,
		tokens:  tokens,
		logins:  logins,
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

// Suspend blocks an account in every service (ADR-0019), for an ADMIN or a
// MODERATOR (see canModerate). The account is marked
// SUSPENDED, so auth refuses its sign-ins and refreshes, then every session
// enters the shared revocation registry, so every service refuses its access
// tokens at once. Sessions are revoked even when the account was already
// suspended, so a call that failed half-way can simply be repeated.
func (s *accountService) Suspend(ctx context.Context, admin TokenClaims, userID uuid.UUID) (*dto.UserResponse, error) {
	if !isStaff(admin) {
		return nil, ErrSuspensionForbidden
	}
	if userID == admin.UserID {
		return nil, ErrSelfSuspension
	}
	user, err := s.adminTarget(ctx, admin, userID)
	if err != nil {
		return nil, fmt.Errorf("suspend account: %w", err)
	}
	if user.Status != model.StatusSuspended {
		user.Status = model.StatusSuspended
		if err := s.users.Update(ctx, user); err != nil {
			return nil, fmt.Errorf("suspend account: %w", err)
		}
	}
	if err := s.RevokeAllSessions(ctx, user.ID); err != nil {
		return nil, fmt.Errorf("suspend account: %w", err)
	}
	s.log.Info("account suspended",
		zap.String("user_id", user.ID.String()), zap.String("actor_id", admin.UserID.String()),
		zap.Strings("actor_roles", admin.Roles), zap.Time("at", s.now().UTC()))
	res := toUserResponse(user)
	return &res, nil
}

// Reinstate lifts a suspension. The account returns to PENDING: its next
// sign-in makes it ACTIVE again, as for any account not yet signed in.
func (s *accountService) Reinstate(ctx context.Context, admin TokenClaims, userID uuid.UUID) (*dto.UserResponse, error) {
	if !isStaff(admin) {
		return nil, ErrSuspensionForbidden
	}
	user, err := s.adminTarget(ctx, admin, userID)
	if err != nil {
		return nil, fmt.Errorf("reinstate account: %w", err)
	}
	if user.Status == model.StatusSuspended {
		user.Status = model.StatusPending
		if err := s.users.Update(ctx, user); err != nil {
			return nil, fmt.Errorf("reinstate account: %w", err)
		}
		s.log.Info("account reinstated",
			zap.String("user_id", user.ID.String()), zap.String("actor_id", admin.UserID.String()),
			zap.Strings("actor_roles", admin.Roles), zap.Time("at", s.now().UTC()))
	}
	res := toUserResponse(user)
	return &res, nil
}

// adminTarget loads the account a staff member acts on: a deleted account
// counts as not found, and a MODERATOR may not act on staff.
func (s *accountService) adminTarget(ctx context.Context, actor TokenClaims, userID uuid.UUID) (*model.User, error) {
	user, err := s.users.GetByID(ctx, userID)
	if errors.Is(err, repository.ErrUserNotFound) || (err == nil && user.Status == model.StatusDeleted) {
		return nil, ErrAccountNotFound
	}
	if err != nil {
		return nil, err
	}
	if !hasRole(actor, model.RoleAdmin) && (userHasRole(user, model.RoleAdmin) || userHasRole(user, model.RoleModerator)) {
		return nil, ErrSuspensionForbidden
	}
	return user, nil
}

func isStaff(claims TokenClaims) bool {
	return hasRole(claims, model.RoleAdmin) || hasRole(claims, model.RoleModerator)
}

func hasRole(claims TokenClaims, role model.UserRole) bool {
	for _, r := range claims.Roles {
		if r == string(role) {
			return true
		}
	}
	return false
}

func userHasRole(user *model.User, role model.UserRole) bool {
	for _, r := range user.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// reauthenticate asks for the password of an email account, or a fresh OTP
// code for an account that signs in by phone only.
func (s *accountService) reauthenticate(ctx context.Context, user *model.User, req dto.DeleteAccountRequest) error {
	switch {
	case user.PasswordHash != nil:
		if req.Password == "" {
			return ErrReauthRequired
		}
		// The same lock as sign-in: a stolen session must not become a way
		// to guess the password.
		account := passwordAccount(user)
		if err := s.logins.Check(ctx, account); err != nil {
			return err
		}
		match, err := argon2id.ComparePasswordAndHash(req.Password, *user.PasswordHash)
		if err != nil {
			return fmt.Errorf("compare password: %w", err)
		}
		if !match {
			if err := s.logins.Fail(ctx, account); err != nil {
				return err
			}
			return ErrInvalidCredentials
		}
		return s.logins.Reset(ctx, account)
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

// passwordAccount names the account of a password user for the login guard:
// its email address, the name sign-in uses.
func passwordAccount(user *model.User) string {
	if user.Email != nil {
		return *user.Email
	}
	return "user:" + user.ID.String()
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
