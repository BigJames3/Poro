package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/auth/internal/config"
	"github.com/poro/auth/internal/dto"
	"github.com/poro/auth/internal/model"
	"github.com/poro/auth/internal/repository"
)

const testPassword = "password123"

func TestNewAuthServiceRejectsBadTTL(t *testing.T) {
	cases := []struct {
		name string
		cfg  *config.Config
	}{
		{name: "access", cfg: &config.Config{JWTAccessTTL: "nope", JWTRefreshTTL: "1h"}},
		{name: "refresh", cfg: &config.Config{JWTAccessTTL: "15m", JWTRefreshTTL: "nope"}},
		{name: "zero", cfg: &config.Config{JWTAccessTTL: "0s", JWTRefreshTTL: "1h"}},
		{name: "grace", cfg: &config.Config{JWTAccessTTL: "15m", JWTRefreshTTL: "1h", JWTRefreshReuseGrace: "-1s"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewAuthService(tc.cfg, newFakeUsers(), newFakeRefresh(), &fakeOTP{}, &fakeTokens{}, zap.NewNop())
			require.Error(t, err)
		})
	}
}

func TestRequestOTP(t *testing.T) {
	otp := &fakeOTP{expiresIn: 300}
	svc := newAuthService(t, newFakeUsers(), newFakeRefresh(), otp, &fakeTokens{})
	res, err := svc.RequestOTP(context.Background(), dto.RequestOTPRequest{Phone: "+2250707070707"})
	require.NoError(t, err)
	require.Equal(t, "otp sent", res.Message)
	require.Equal(t, 300, res.ExpiresIn)
	require.Equal(t, []string{"+2250707070707"}, otp.phones)

	otp.err = errors.New("sms down")
	_, err = svc.RequestOTP(context.Background(), dto.RequestOTPRequest{Phone: "+2250707070707"})
	require.Error(t, err)
}

func TestVerifyOTPCreatesPersonalUserAndSession(t *testing.T) {
	users := newFakeUsers()
	refresh := newFakeRefresh()
	tokens := &fakeTokens{}
	svc := newAuthService(t, users, refresh, &fakeOTP{}, tokens)
	res, err := svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: "+2250707070707", Code: "123456"}, "Poro", "127.0.0.1")
	require.NoError(t, err)
	require.Equal(t, "Bearer", res.TokenType)
	require.Equal(t, "access-token", res.AccessToken)
	require.Equal(t, "refresh-token", res.RefreshToken)
	require.Equal(t, 900, res.ExpiresIn)
	require.NotNil(t, res.User.Phone)
	require.Equal(t, []string{"PERSONAL"}, res.User.Roles)
	require.Equal(t, model.StatusActive, model.UserStatus(res.User.Status))

	require.Len(t, refresh.items, 1)
	session := refresh.items[0]
	require.Equal(t, session.ID, session.FamilyID)
	require.Equal(t, session.FamilyID, tokens.lastSession)
	require.Equal(t, []string{"PERSONAL"}, tokens.lastRoles)
	require.Equal(t, "Poro", *session.UserAgent)
	require.Equal(t, "127.0.0.1", *session.IPAddress)
	require.NotNil(t, users.byID[session.UserID].LastLoginAt)
}

func TestVerifyOTPActivatesPendingUser(t *testing.T) {
	users := newFakeUsers()
	phone := "+2250707070707"
	id := mustID(t)
	users.byID[id] = &model.User{ID: id, Phone: &phone, Roles: []model.UserRole{model.RolePersonal, model.RoleCreator}, Status: model.StatusPending, CreatedAt: time.Now().UTC()}
	refresh := newFakeRefresh()
	svc := newAuthService(t, users, refresh, &fakeOTP{}, &fakeTokens{})
	res, err := svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: phone, Code: "123456"}, "", "")
	require.NoError(t, err)
	require.Equal(t, string(model.StatusActive), res.User.Status)
	require.Equal(t, []string{"PERSONAL", "CREATOR"}, res.User.Roles)
	require.Nil(t, refresh.items[0].UserAgent)
	require.Nil(t, refresh.items[0].IPAddress)
}

func TestVerifyOTPErrors(t *testing.T) {
	phone := "+2250707070707"
	id := mustID(t)
	active := &model.User{ID: id, Phone: &phone, Roles: []model.UserRole{model.RolePersonal}, Status: model.StatusActive, CreatedAt: time.Now().UTC()}

	t.Run("otp rejected", func(t *testing.T) {
		svc := newAuthService(t, newFakeUsers(), newFakeRefresh(), &fakeOTP{err: ErrOTPInvalid}, &fakeTokens{})
		_, err := svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: phone, Code: "000000"}, "", "")
		require.ErrorIs(t, err, ErrOTPInvalid)
	})

	t.Run("lookup failed", func(t *testing.T) {
		users := newFakeUsers()
		users.getByPhoneErr = errors.New("db")
		svc := newAuthService(t, users, newFakeRefresh(), &fakeOTP{}, &fakeTokens{})
		_, err := svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: phone, Code: "123456"}, "", "")
		require.Error(t, err)
		require.NotErrorIs(t, err, repository.ErrUserNotFound)
	})

	t.Run("create failed", func(t *testing.T) {
		users := newFakeUsers()
		users.createErr = errors.New("db")
		svc := newAuthService(t, users, newFakeRefresh(), &fakeOTP{}, &fakeTokens{})
		_, err := svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: phone, Code: "123456"}, "", "")
		require.Error(t, err)
	})

	t.Run("race then found", func(t *testing.T) {
		users := newFakeUsers()
		users.byID[id] = active
		users.revealPhoneAfterCreate = true
		users.createErr = repository.ErrUserAlreadyExists
		svc := newAuthService(t, users, newFakeRefresh(), &fakeOTP{}, &fakeTokens{})
		res, err := svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: phone, Code: "123456"}, "", "")
		require.NoError(t, err)
		require.Equal(t, id.String(), res.User.ID)
	})

	t.Run("race then lookup failed", func(t *testing.T) {
		users := newFakeUsers()
		users.revealPhoneAfterCreate = true
		users.createErr = repository.ErrUserAlreadyExists
		users.getByPhoneErr = errors.New("db")
		svc := newAuthService(t, users, newFakeRefresh(), &fakeOTP{}, &fakeTokens{})
		_, err := svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: phone, Code: "123456"}, "", "")
		require.Error(t, err)
	})

	t.Run("blocked", func(t *testing.T) {
		users := newFakeUsers()
		users.byID[id] = &model.User{ID: id, Phone: &phone, Status: model.StatusSuspended, CreatedAt: time.Now().UTC()}
		svc := newAuthService(t, users, newFakeRefresh(), &fakeOTP{}, &fakeTokens{})
		_, err := svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: phone, Code: "123456"}, "", "")
		require.ErrorIs(t, err, ErrAccountBlocked)
	})

	t.Run("activate failed", func(t *testing.T) {
		users := newFakeUsers()
		users.byID[id] = &model.User{ID: id, Phone: &phone, Status: model.StatusPending, CreatedAt: time.Now().UTC()}
		users.updateErr = errors.New("db")
		svc := newAuthService(t, users, newFakeRefresh(), &fakeOTP{}, &fakeTokens{})
		_, err := svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: phone, Code: "123456"}, "", "")
		require.Error(t, err)
	})

	t.Run("session failed", func(t *testing.T) {
		svc := newAuthService(t, newFakeUsers(), newFakeRefresh(), &fakeOTP{}, &fakeTokens{failAccess: true})
		_, err := svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: phone, Code: "123456"}, "", "")
		require.Error(t, err)
	})
}

func TestRegisterEmail(t *testing.T) {
	users := newFakeUsers()
	svc := newAuthService(t, users, newFakeRefresh(), &fakeOTP{}, &fakeTokens{})
	res, err := svc.RegisterEmail(context.Background(), dto.RegisterEmailRequest{
		Email: "  Ada@Poro.App ", Password: testPassword, FullName: "Ada Lovelace",
	}, "Poro", "")
	require.NoError(t, err)
	require.Equal(t, "ada@poro.app", *res.User.Email)
	require.Equal(t, []string{"PERSONAL"}, res.User.Roles)
	require.Equal(t, "Bearer", res.TokenType)
	for _, user := range users.byID {
		require.NotNil(t, user.PasswordHash)
		params, _, _, err := argon2id.DecodeHash(*user.PasswordHash)
		require.NoError(t, err)
		require.Equal(t, uint32(19*1024), params.Memory)
		match, err := argon2id.ComparePasswordAndHash(testPassword, *user.PasswordHash)
		require.NoError(t, err)
		require.True(t, match)
	}

	_, err = svc.RegisterEmail(context.Background(), dto.RegisterEmailRequest{
		Email: "ADA@poro.app", Password: testPassword, FullName: "Ada Lovelace",
	}, "", "")
	require.ErrorIs(t, err, ErrEmailTaken)

	users.createErr = errors.New("db")
	_, err = svc.RegisterEmail(context.Background(), dto.RegisterEmailRequest{
		Email: "other@poro.app", Password: testPassword, FullName: "Ada Lovelace",
	}, "", "")
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrEmailTaken)
}

func TestLoginEmail(t *testing.T) {
	hash := hashPassword(t)
	email := "ada@poro.app"
	id := mustID(t)
	users := newFakeUsers()
	users.byID[id] = &model.User{
		ID: id, Email: &email, PasswordHash: &hash,
		Roles: []model.UserRole{model.RolePersonal}, Status: model.StatusActive, CreatedAt: time.Now().UTC(),
	}
	svc := newAuthService(t, users, newFakeRefresh(), &fakeOTP{}, &fakeTokens{})
	res, err := svc.LoginEmail(context.Background(), dto.LoginEmailRequest{Email: "Ada@Poro.app", Password: testPassword}, "", "10.0.0.8")
	require.NoError(t, err)
	require.Equal(t, id.String(), res.User.ID)

	_, err = svc.LoginEmail(context.Background(), dto.LoginEmailRequest{Email: email, Password: "wrong-password"}, "", "")
	require.ErrorIs(t, err, ErrInvalidCredentials)

	_, err = svc.LoginEmail(context.Background(), dto.LoginEmailRequest{Email: "missing@poro.app", Password: testPassword}, "", "")
	require.ErrorIs(t, err, ErrInvalidCredentials)

	users.byID[id].Status = model.StatusDeleted
	_, err = svc.LoginEmail(context.Background(), dto.LoginEmailRequest{Email: email, Password: "wrong-password"}, "", "")
	require.ErrorIs(t, err, ErrInvalidCredentials, "a wrong password must not reveal the account status")
	_, err = svc.LoginEmail(context.Background(), dto.LoginEmailRequest{Email: email, Password: testPassword}, "", "")
	require.ErrorIs(t, err, ErrAccountBlocked)
	users.byID[id].Status = model.StatusActive

	users.byID[id].PasswordHash = nil
	_, err = svc.LoginEmail(context.Background(), dto.LoginEmailRequest{Email: email, Password: testPassword}, "", "")
	require.ErrorIs(t, err, ErrInvalidCredentials)
	users.byID[id].PasswordHash = &hash

	bad := "not-a-hash"
	users.byID[id].PasswordHash = &bad
	_, err = svc.LoginEmail(context.Background(), dto.LoginEmailRequest{Email: email, Password: testPassword}, "", "")
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrInvalidCredentials)
	users.byID[id].PasswordHash = &hash

	users.byID[id].Status = model.StatusPending
	res, err = svc.LoginEmail(context.Background(), dto.LoginEmailRequest{Email: email, Password: testPassword}, "", "")
	require.NoError(t, err)
	require.Equal(t, string(model.StatusActive), res.User.Status)

	users.getByEmailErr = errors.New("db")
	_, err = svc.LoginEmail(context.Background(), dto.LoginEmailRequest{Email: email, Password: testPassword}, "", "")
	require.Error(t, err)
}

func TestRefreshRotatesWithinFamily(t *testing.T) {
	id := mustID(t)
	users := newFakeUsers()
	users.byID[id] = &model.User{ID: id, Roles: []model.UserRole{model.RolePersonal, model.RoleBusiness}, Status: model.StatusActive, CreatedAt: time.Now().UTC()}
	ua, ip := "Poro", "192.0.2.10"
	refresh := newFakeRefresh()
	old := refresh.seed(t, id, "old-refresh")
	old.UserAgent, old.IPAddress = &ua, &ip
	tokens := &fakeTokens{refresh: "new-refresh"}
	svc := newAuthService(t, users, refresh, &fakeOTP{}, tokens)

	res, err := svc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "old-refresh"})
	require.NoError(t, err)
	require.Equal(t, "access-token", res.AccessToken)
	require.Equal(t, "new-refresh", res.RefreshToken)
	require.Equal(t, "Bearer", res.TokenType)
	require.Equal(t, 900, res.ExpiresIn)

	require.Len(t, refresh.items, 2)
	stored, next := refresh.items[0], refresh.items[1]
	require.NotNil(t, stored.RevokedAt)
	require.Equal(t, next.ID, *stored.ReplacedBy)
	require.Equal(t, stored.FamilyID, next.FamilyID)
	require.Nil(t, next.RevokedAt)
	require.Equal(t, ua, *next.UserAgent)
	require.Equal(t, ip, *next.IPAddress)
	require.Equal(t, stored.FamilyID, tokens.lastSession)
	require.Equal(t, []string{"PERSONAL", "BUSINESS"}, tokens.lastRoles)
}

func TestRefreshReuseRevokesFamily(t *testing.T) {
	id := mustID(t)
	users := newFakeUsers()
	users.byID[id] = &model.User{ID: id, Roles: []model.UserRole{model.RolePersonal}, Status: model.StatusActive, CreatedAt: time.Now().UTC()}
	refresh := newFakeRefresh()
	refresh.seed(t, id, "t1")
	tokens := &fakeTokens{refresh: "t2"}
	svc := newAuthServiceWithGrace(t, users, refresh, tokens, "0s")

	_, err := svc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "t1"})
	require.NoError(t, err)

	_, err = svc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "t1"})
	require.ErrorIs(t, err, ErrRefreshTokenReused)
	for _, token := range refresh.items {
		require.NotNil(t, token.RevokedAt, "every token of the family must be revoked")
	}

	_, err = svc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "t2"})
	require.ErrorIs(t, err, ErrRefreshTokenInvalid, "a fully revoked family is simply invalid")
}

func TestRefreshGraceAllowsLostResponseRetry(t *testing.T) {
	id := mustID(t)
	users := newFakeUsers()
	users.byID[id] = &model.User{ID: id, Roles: []model.UserRole{model.RolePersonal}, Status: model.StatusActive, CreatedAt: time.Now().UTC()}
	refresh := newFakeRefresh()
	refresh.seed(t, id, "t1")
	tokens := &fakeTokens{refresh: "t2"}
	svc := newAuthServiceWithGrace(t, users, refresh, tokens, "30s")

	_, err := svc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "t1"})
	require.NoError(t, err)

	tokens.refresh = "t3"
	res, err := svc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "t1"})
	require.NoError(t, err, "a retry inside the grace window must succeed")
	require.Equal(t, "t3", res.RefreshToken)

	t2 := refresh.byHash(HashToken("t2"))
	require.NotNil(t, t2.RevokedAt, "the unseen successor is superseded")
	require.Nil(t, t2.ReplacedBy)
	t3 := refresh.byHash(HashToken("t3"))
	require.Nil(t, t3.RevokedAt)

	_, err = svc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "t2"})
	require.ErrorIs(t, err, ErrRefreshTokenReused, "using the superseded token means two holders: revoke the session")
	require.NotNil(t, refresh.byHash(HashToken("t3")).RevokedAt)
}

func TestRefreshGraceRejectsRotatedSuccessor(t *testing.T) {
	id := mustID(t)
	users := newFakeUsers()
	users.byID[id] = &model.User{ID: id, Roles: []model.UserRole{model.RolePersonal}, Status: model.StatusActive, CreatedAt: time.Now().UTC()}
	refresh := newFakeRefresh()
	refresh.seed(t, id, "t1")
	tokens := &fakeTokens{refresh: "t2"}
	svc := newAuthServiceWithGrace(t, users, refresh, tokens, "30s")

	_, err := svc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "t1"})
	require.NoError(t, err)
	tokens.refresh = "t3"
	_, err = svc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "t2"})
	require.NoError(t, err)

	_, err = svc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "t1"})
	require.ErrorIs(t, err, ErrRefreshTokenReused, "t2 was already used, so t1 cannot be a lost-response retry")
}

func TestRefreshErrors(t *testing.T) {
	id := mustID(t)
	valid := func(t *testing.T) (*fakeUsers, *fakeRefresh) {
		users := newFakeUsers()
		users.byID[id] = &model.User{ID: id, Roles: []model.UserRole{model.RolePersonal}, Status: model.StatusActive, CreatedAt: time.Now().UTC()}
		refresh := newFakeRefresh()
		refresh.seed(t, id, "old-refresh")
		return users, refresh
	}
	call := func(t *testing.T, users *fakeUsers, refresh *fakeRefresh, tokens *fakeTokens) error {
		svc := newAuthService(t, users, refresh, &fakeOTP{}, tokens)
		_, err := svc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "old-refresh"})
		return err
	}

	t.Run("unknown", func(t *testing.T) {
		require.ErrorIs(t, call(t, newFakeUsers(), newFakeRefresh(), &fakeTokens{}), ErrRefreshTokenInvalid)
	})

	t.Run("store error", func(t *testing.T) {
		users, refresh := valid(t)
		refresh.getErr = errors.New("db")
		err := call(t, users, refresh, &fakeTokens{})
		require.Error(t, err)
		require.NotErrorIs(t, err, ErrRefreshTokenInvalid)
	})

	t.Run("revoked by logout", func(t *testing.T) {
		users, refresh := valid(t)
		now := time.Now().UTC()
		refresh.items[0].RevokedAt = &now
		require.ErrorIs(t, call(t, users, refresh, &fakeTokens{}), ErrRefreshTokenInvalid)
	})

	t.Run("expired", func(t *testing.T) {
		users, refresh := valid(t)
		refresh.items[0].ExpiresAt = time.Now().UTC().Add(-time.Minute)
		require.ErrorIs(t, call(t, users, refresh, &fakeTokens{}), ErrRefreshTokenInvalid)
	})

	t.Run("user missing", func(t *testing.T) {
		_, refresh := valid(t)
		require.ErrorIs(t, call(t, newFakeUsers(), refresh, &fakeTokens{}), ErrRefreshTokenInvalid)
	})

	t.Run("user lookup failed", func(t *testing.T) {
		users, refresh := valid(t)
		users.getByIDErr = errors.New("db")
		require.Error(t, call(t, users, refresh, &fakeTokens{}))
	})

	t.Run("blocked rolls back", func(t *testing.T) {
		users, refresh := valid(t)
		users.byID[id].Status = model.StatusSuspended
		require.ErrorIs(t, call(t, users, refresh, &fakeTokens{}), ErrAccountBlocked)
		require.Len(t, refresh.items, 1)
		require.Nil(t, refresh.items[0].RevokedAt)
	})

	t.Run("access failed rolls back", func(t *testing.T) {
		users, refresh := valid(t)
		require.Error(t, call(t, users, refresh, &fakeTokens{failAccess: true}))
		require.Len(t, refresh.items, 1)
		require.Nil(t, refresh.items[0].RevokedAt)
	})

	t.Run("refresh generation failed", func(t *testing.T) {
		users, refresh := valid(t)
		require.Error(t, call(t, users, refresh, &fakeTokens{failRefresh: true}))
	})

	t.Run("store create failed", func(t *testing.T) {
		users, refresh := valid(t)
		refresh.createErr = errors.New("db")
		require.Error(t, call(t, users, refresh, &fakeTokens{}))
	})

	t.Run("mark replaced failed", func(t *testing.T) {
		users, refresh := valid(t)
		refresh.revokeErr = errors.New("db")
		require.Error(t, call(t, users, refresh, &fakeTokens{}))
	})
}

func TestLogoutRevokesSession(t *testing.T) {
	id := mustID(t)
	refresh := newFakeRefresh()
	current := refresh.seed(t, id, "current")
	other := refresh.seed(t, id, "other-device")
	tokens := &fakeTokens{}
	svc := newAuthService(t, newFakeUsers(), refresh, &fakeOTP{}, tokens)
	claims := TokenClaims{UserID: id, SessionID: current.FamilyID, TokenID: "jti-1", Exp: time.Now().Add(time.Minute)}

	require.NoError(t, svc.Logout(context.Background(), claims))
	require.Equal(t, []string{"jti-1"}, tokens.blacklisted)
	require.NotNil(t, current.RevokedAt)
	require.Nil(t, other.RevokedAt, "other devices stay signed in")

	require.NoError(t, svc.Logout(context.Background(), claims), "logout is idempotent")

	tokens.failBlacklist = true
	require.Error(t, svc.Logout(context.Background(), claims))
	tokens.failBlacklist = false

	refresh.revokeErr = errors.New("db")
	require.Error(t, svc.Logout(context.Background(), claims))
}

func TestMe(t *testing.T) {
	id := mustID(t)
	phone := "+2250707070707"
	users := newFakeUsers()
	created := time.Date(2026, 9, 27, 11, 0, 0, 0, time.UTC)
	users.byID[id] = &model.User{
		ID: id, Phone: &phone, Roles: []model.UserRole{model.RolePersonal, model.RoleCreator}, Status: model.StatusActive,
		Language: "fr", CreatedAt: created,
	}
	svc := newAuthService(t, users, newFakeRefresh(), &fakeOTP{}, &fakeTokens{})
	res, err := svc.Me(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, id.String(), res.ID)
	require.Equal(t, []string{"PERSONAL", "CREATOR"}, res.Roles)
	require.Equal(t, "fr", res.Language)
	require.Equal(t, "2026-09-27T11:00:00Z", res.CreatedAt)

	_, err = svc.Me(context.Background(), mustID(t))
	require.ErrorIs(t, err, ErrAccountNotFound)

	users.getByIDErr = errors.New("db")
	_, err = svc.Me(context.Background(), id)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrAccountNotFound)
}

func TestIssueSessionFailures(t *testing.T) {
	phone := "+2250707070707"
	refresh := newFakeRefresh()
	refresh.createErr = errors.New("db")
	svc := newAuthService(t, newFakeUsers(), refresh, &fakeOTP{}, &fakeTokens{})
	_, err := svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: phone, Code: "123456"}, "", "")
	require.Error(t, err)

	users := newFakeUsers()
	users.lastLoginErr = errors.New("db")
	svc = newAuthService(t, users, newFakeRefresh(), &fakeOTP{}, &fakeTokens{})
	_, err = svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: phone, Code: "123456"}, "", "")
	require.Error(t, err)

	svc = newAuthService(t, newFakeUsers(), newFakeRefresh(), &fakeOTP{}, &fakeTokens{failRefresh: true})
	_, err = svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: phone, Code: "123456"}, "", "")
	require.Error(t, err)
}

func TestNormalizeEmail(t *testing.T) {
	require.Equal(t, "ada@poro.app", NormalizeEmail("  ADA@Poro.App\t"))
}

func newAuthService(t *testing.T, users *fakeUsers, refresh *fakeRefresh, otp OTPService, tokens TokenService) AuthService {
	t.Helper()
	svc, err := NewAuthService(
		&config.Config{JWTAccessTTL: "15m", JWTRefreshTTL: "720h", JWTRefreshReuseGrace: "30s"},
		users, refresh, otp, tokens, zap.NewNop(),
	)
	require.NoError(t, err)
	return svc
}

func newAuthServiceWithGrace(t *testing.T, users *fakeUsers, refresh *fakeRefresh, tokens TokenService, grace string) AuthService {
	t.Helper()
	svc, err := NewAuthService(
		&config.Config{JWTAccessTTL: "15m", JWTRefreshTTL: "720h", JWTRefreshReuseGrace: grace},
		users, refresh, &fakeOTP{}, tokens, zap.NewNop(),
	)
	require.NoError(t, err)
	return svc
}

func mustID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	require.NoError(t, err)
	return id
}

var (
	passwordOnce sync.Once
	passwordHash string
	passwordErr  error
)

func hashPassword(t *testing.T) string {
	t.Helper()
	passwordOnce.Do(func() {
		passwordHash, passwordErr = argon2id.CreateHash(testPassword, passwordParams)
	})
	require.NoError(t, passwordErr)
	return passwordHash
}

type fakeUsers struct {
	byID                   map[uuid.UUID]*model.User
	createErr              error
	getByPhoneErr          error
	getByEmailErr          error
	getByIDErr             error
	updateErr              error
	lastLoginErr           error
	revealPhoneAfterCreate bool
	createAttempted        bool
}

func newFakeUsers() *fakeUsers {
	return &fakeUsers{byID: map[uuid.UUID]*model.User{}}
}

func (f *fakeUsers) Create(_ context.Context, user *model.User) error {
	f.createAttempted = true
	if f.createErr != nil {
		return f.createErr
	}
	for _, existing := range f.byID {
		if user.Phone != nil && existing.Phone != nil && *existing.Phone == *user.Phone {
			return repository.ErrUserAlreadyExists
		}
		if user.Email != nil && existing.Email != nil && *existing.Email == *user.Email {
			return repository.ErrUserAlreadyExists
		}
	}
	if user.ID == uuid.Nil {
		user.ID = uuid.Must(uuid.NewV7())
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now().UTC()
	}
	f.byID[user.ID] = user
	return nil
}

func (f *fakeUsers) GetByID(_ context.Context, id uuid.UUID) (*model.User, error) {
	if f.getByIDErr != nil {
		return nil, f.getByIDErr
	}
	user, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrUserNotFound
	}
	return user, nil
}

func (f *fakeUsers) GetByPhone(_ context.Context, phone string) (*model.User, error) {
	if f.revealPhoneAfterCreate && !f.createAttempted {
		return nil, repository.ErrUserNotFound
	}
	if f.getByPhoneErr != nil {
		return nil, f.getByPhoneErr
	}
	for _, user := range f.byID {
		if user.Phone != nil && *user.Phone == phone {
			return user, nil
		}
	}
	return nil, repository.ErrUserNotFound
}

func (f *fakeUsers) GetByEmail(_ context.Context, email string) (*model.User, error) {
	if f.getByEmailErr != nil {
		return nil, f.getByEmailErr
	}
	for _, user := range f.byID {
		if user.Email != nil && *user.Email == email {
			return user, nil
		}
	}
	return nil, repository.ErrUserNotFound
}

func (f *fakeUsers) Update(_ context.Context, user *model.User) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	if _, ok := f.byID[user.ID]; !ok {
		return repository.ErrUserNotFound
	}
	f.byID[user.ID] = user
	return nil
}

func (f *fakeUsers) UpdateLastLogin(_ context.Context, id uuid.UUID) error {
	if f.lastLoginErr != nil {
		return f.lastLoginErr
	}
	user, ok := f.byID[id]
	if !ok {
		return repository.ErrUserNotFound
	}
	now := time.Now().UTC()
	user.LastLoginAt = &now
	return nil
}

func (f *fakeUsers) SoftDelete(context.Context, uuid.UUID) error { return nil }

func (f *fakeUsers) ExistsByPhone(ctx context.Context, phone string) (bool, error) {
	_, err := f.GetByPhone(ctx, phone)
	if errors.Is(err, repository.ErrUserNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (f *fakeUsers) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	_, err := f.GetByEmail(ctx, email)
	if errors.Is(err, repository.ErrUserNotFound) {
		return false, nil
	}
	return err == nil, err
}

// fakeRefresh is an in-memory refresh token store. InTx restores a snapshot on error.
type fakeRefresh struct {
	items     []*model.RefreshToken
	createErr error
	getErr    error
	revokeErr error
}

func newFakeRefresh() *fakeRefresh { return &fakeRefresh{} }

func (f *fakeRefresh) seed(t *testing.T, userID uuid.UUID, raw string) *model.RefreshToken {
	t.Helper()
	id := mustID(t)
	token := &model.RefreshToken{
		ID: id, UserID: userID, FamilyID: id, TokenHash: HashToken(raw),
		ExpiresAt: time.Now().UTC().Add(time.Hour), CreatedAt: time.Now().UTC(),
	}
	f.items = append(f.items, token)
	return token
}

func (f *fakeRefresh) byHash(hash string) *model.RefreshToken {
	for _, token := range f.items {
		if token.TokenHash == hash {
			return token
		}
	}
	return nil
}

func (f *fakeRefresh) Create(_ context.Context, token *model.RefreshToken) error {
	if f.createErr != nil {
		return f.createErr
	}
	if token.ID == uuid.Nil {
		token.ID = uuid.Must(uuid.NewV7())
	}
	if token.FamilyID == uuid.Nil {
		token.FamilyID = token.ID
	}
	f.items = append(f.items, token)
	return nil
}

func (f *fakeRefresh) GetByHash(_ context.Context, hash string) (*model.RefreshToken, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if token := f.byHash(hash); token != nil {
		return token, nil
	}
	return nil, repository.ErrRefreshTokenNotFound
}

func (f *fakeRefresh) GetByHashForUpdate(ctx context.Context, hash string) (*model.RefreshToken, error) {
	return f.GetByHash(ctx, hash)
}

func (f *fakeRefresh) GetByIDForUpdate(_ context.Context, id uuid.UUID) (*model.RefreshToken, error) {
	for _, token := range f.items {
		if token.ID == id {
			return token, nil
		}
	}
	return nil, repository.ErrRefreshTokenNotFound
}

func (f *fakeRefresh) MarkReplaced(_ context.Context, id, replacedBy uuid.UUID, at time.Time) error {
	if f.revokeErr != nil {
		return f.revokeErr
	}
	for _, token := range f.items {
		if token.ID == id {
			if token.RevokedAt == nil {
				token.RevokedAt = &at
			}
			next := replacedBy
			token.ReplacedBy = &next
			return nil
		}
	}
	return repository.ErrRefreshTokenNotFound
}

func (f *fakeRefresh) Revoke(_ context.Context, id uuid.UUID, at time.Time) error {
	if f.revokeErr != nil {
		return f.revokeErr
	}
	for _, token := range f.items {
		if token.ID == id {
			if token.RevokedAt == nil {
				token.RevokedAt = &at
			}
			return nil
		}
	}
	return repository.ErrRefreshTokenNotFound
}

func (f *fakeRefresh) revokeWhere(match func(*model.RefreshToken) bool, at time.Time) (int64, error) {
	if f.revokeErr != nil {
		return 0, f.revokeErr
	}
	var n int64
	for _, token := range f.items {
		if token.RevokedAt == nil && match(token) {
			revokedAt := at
			token.RevokedAt = &revokedAt
			n++
		}
	}
	return n, nil
}

func (f *fakeRefresh) InTx(_ context.Context, fn func(tx repository.RefreshTokenTx) error) error {
	snapshot := make([]model.RefreshToken, len(f.items))
	for i, token := range f.items {
		snapshot[i] = *token
	}
	if err := fn(&fakeRefreshTx{f}); err != nil {
		f.items = f.items[:len(snapshot)]
		for i := range snapshot {
			*f.items[i] = snapshot[i]
		}
		return err
	}
	return nil
}

func (f *fakeRefresh) RevokeFamily(_ context.Context, userID, familyID uuid.UUID) (int64, error) {
	return f.revokeWhere(func(t *model.RefreshToken) bool {
		return t.UserID == userID && t.FamilyID == familyID
	}, time.Now().UTC())
}

func (f *fakeRefresh) RevokeAllForUser(context.Context, uuid.UUID) error { return nil }

func (f *fakeRefresh) DeleteExpired(context.Context) (int64, error) { return 0, nil }

type fakeRefreshTx struct{ *fakeRefresh }

func (tx *fakeRefreshTx) RevokeFamily(_ context.Context, familyID uuid.UUID, at time.Time) (int64, error) {
	return tx.revokeWhere(func(t *model.RefreshToken) bool { return t.FamilyID == familyID }, at)
}

type fakeOTP struct {
	expiresIn int
	err       error
	phones    []string
}

func (f *fakeOTP) RequestOTP(_ context.Context, phone string) (int, error) {
	f.phones = append(f.phones, phone)
	if f.err != nil {
		return 0, f.err
	}
	return f.expiresIn, nil
}

func (f *fakeOTP) VerifyOTP(context.Context, string, string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return true, nil
}

type fakeTokens struct {
	refresh       string
	failAccess    bool
	failRefresh   bool
	failBlacklist bool
	blacklisted   []string
	lastRoles     []string
	lastSession   uuid.UUID
}

func (f *fakeTokens) GenerateAccessToken(_ uuid.UUID, roles []string, sessionID uuid.UUID) (string, error) {
	if f.failAccess {
		return "", errors.New("sign")
	}
	f.lastRoles, f.lastSession = roles, sessionID
	return "access-token", nil
}

func (f *fakeTokens) GenerateRefreshToken() (string, string, error) {
	if f.failRefresh {
		return "", "", errors.New("refresh")
	}
	raw := f.refresh
	if raw == "" {
		raw = "refresh-token"
	}
	return raw, HashToken(raw), nil
}

func (f *fakeTokens) ValidateAccessToken(string) (*TokenClaims, error) {
	return nil, ErrInvalidToken
}

func (f *fakeTokens) HashToken(token string) string { return HashToken(token) }

func (f *fakeTokens) BlacklistToken(_ context.Context, tokenID string, _ time.Duration) error {
	if f.failBlacklist {
		return errors.New("redis")
	}
	f.blacklisted = append(f.blacklisted, tokenID)
	return nil
}

func (f *fakeTokens) IsBlacklisted(context.Context, string) (bool, error) { return false, nil }

func (f *fakeTokens) JWKS() JWKSet { return JWKSet{} }
