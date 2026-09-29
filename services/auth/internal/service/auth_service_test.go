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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewAuthService(tc.cfg, newFakeUsers(), &fakeRefresh{}, &fakeOTP{}, &fakeTokens{}, zap.NewNop())
			require.Error(t, err)
		})
	}
}

func TestRequestOTP(t *testing.T) {
	otp := &fakeOTP{expiresIn: 300}
	svc := newAuthService(t, newFakeUsers(), &fakeRefresh{}, otp, &fakeTokens{})
	res, err := svc.RequestOTP(context.Background(), dto.RequestOTPRequest{Phone: "+2250707070707"})
	require.NoError(t, err)
	require.Equal(t, "otp sent", res.Message)
	require.Equal(t, 300, res.ExpiresIn)
	require.Equal(t, []string{"+2250707070707"}, otp.phones)

	otp.err = errors.New("sms down")
	_, err = svc.RequestOTP(context.Background(), dto.RequestOTPRequest{Phone: "+2250707070707"})
	require.Error(t, err)
}

func TestVerifyOTPCreatesUser(t *testing.T) {
	users := newFakeUsers()
	refresh := &fakeRefresh{}
	tokens := fakeTokens{}
	svc := newAuthService(t, users, refresh, &fakeOTP{}, &tokens)
	res, err := svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: "+2250707070707", Code: "123456"}, "Poro", "127.0.0.1")
	require.NoError(t, err)
	require.Equal(t, "Bearer", res.TokenType)
	require.Equal(t, "access-token", res.AccessToken)
	require.Equal(t, "refresh-token", res.RefreshToken)
	require.Equal(t, 900, res.ExpiresIn)
	require.NotNil(t, res.User.Phone)
	require.Equal(t, model.StatusActive, model.UserStatus(res.User.Status))
	require.Len(t, refresh.items, 1)
	require.Equal(t, "Poro", *refresh.items[0].UserAgent)
	require.Equal(t, "127.0.0.1", *refresh.items[0].IPAddress)
	require.NotNil(t, users.byID[refresh.items[0].UserID].LastLoginAt)
}

func TestVerifyOTPActivatesPendingUser(t *testing.T) {
	users := newFakeUsers()
	phone := "+2250707070707"
	id := mustID(t)
	users.byID[id] = &model.User{ID: id, Phone: &phone, Role: model.RolePersonal, Status: model.StatusPending, CreatedAt: time.Now().UTC()}
	refresh := &fakeRefresh{}
	svc := newAuthService(t, users, refresh, &fakeOTP{}, &fakeTokens{})
	res, err := svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: phone, Code: "123456"}, "", "")
	require.NoError(t, err)
	require.Equal(t, string(model.StatusActive), res.User.Status)
	require.Equal(t, phone, *res.User.Phone)
	require.NotNil(t, users.byID[id].LastLoginAt)
	require.Nil(t, refresh.items[0].UserAgent)
	require.Nil(t, refresh.items[0].IPAddress)
}

func TestVerifyOTPErrors(t *testing.T) {
	phone := "+2250707070707"
	id := mustID(t)
	active := &model.User{ID: id, Phone: &phone, Role: model.RolePersonal, Status: model.StatusActive, CreatedAt: time.Now().UTC()}

	t.Run("otp rejected", func(t *testing.T) {
		svc := newAuthService(t, newFakeUsers(), &fakeRefresh{}, &fakeOTP{err: ErrOTPInvalid}, &fakeTokens{})
		_, err := svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: phone, Code: "000000"}, "", "")
		require.ErrorIs(t, err, ErrOTPInvalid)
	})

	t.Run("lookup failed", func(t *testing.T) {
		users := newFakeUsers()
		users.getByPhoneErr = errors.New("db")
		svc := newAuthService(t, users, &fakeRefresh{}, &fakeOTP{}, &fakeTokens{})
		_, err := svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: phone, Code: "123456"}, "", "")
		require.Error(t, err)
		require.NotErrorIs(t, err, repository.ErrUserNotFound)
	})

	t.Run("create failed", func(t *testing.T) {
		users := newFakeUsers()
		users.createErr = errors.New("db")
		svc := newAuthService(t, users, &fakeRefresh{}, &fakeOTP{}, &fakeTokens{})
		_, err := svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: phone, Code: "123456"}, "", "")
		require.Error(t, err)
	})

	t.Run("race then found", func(t *testing.T) {
		users := newFakeUsers()
		users.byID[id] = active
		users.revealPhoneAfterCreate = true
		users.createErr = repository.ErrUserAlreadyExists
		svc := newAuthService(t, users, &fakeRefresh{}, &fakeOTP{}, &fakeTokens{})
		res, err := svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: phone, Code: "123456"}, "", "")
		require.NoError(t, err)
		require.Equal(t, id.String(), res.User.ID)
	})

	t.Run("race then lookup failed", func(t *testing.T) {
		users := newFakeUsers()
		users.revealPhoneAfterCreate = true
		users.createErr = repository.ErrUserAlreadyExists
		users.getByPhoneErr = errors.New("db")
		svc := newAuthService(t, users, &fakeRefresh{}, &fakeOTP{}, &fakeTokens{})
		_, err := svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: phone, Code: "123456"}, "", "")
		require.Error(t, err)
	})

	t.Run("blocked", func(t *testing.T) {
		users := newFakeUsers()
		users.byID[id] = &model.User{ID: id, Phone: &phone, Status: model.StatusSuspended, CreatedAt: time.Now().UTC()}
		svc := newAuthService(t, users, &fakeRefresh{}, &fakeOTP{}, &fakeTokens{})
		_, err := svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: phone, Code: "123456"}, "", "")
		require.ErrorIs(t, err, ErrAccountBlocked)
	})

	t.Run("activate failed", func(t *testing.T) {
		users := newFakeUsers()
		users.byID[id] = &model.User{ID: id, Phone: &phone, Status: model.StatusPending, CreatedAt: time.Now().UTC()}
		users.updateErr = errors.New("db")
		svc := newAuthService(t, users, &fakeRefresh{}, &fakeOTP{}, &fakeTokens{})
		_, err := svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: phone, Code: "123456"}, "", "")
		require.Error(t, err)
	})

	t.Run("session failed", func(t *testing.T) {
		svc := newAuthService(t, newFakeUsers(), &fakeRefresh{}, &fakeOTP{}, &fakeTokens{failAccess: true})
		_, err := svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: phone, Code: "123456"}, "", "")
		require.Error(t, err)
	})
}

func TestRegisterEmail(t *testing.T) {
	users := newFakeUsers()
	svc := newAuthService(t, users, &fakeRefresh{}, &fakeOTP{}, &fakeTokens{})
	res, err := svc.RegisterEmail(context.Background(), dto.RegisterEmailRequest{
		Email: "ada@poro.app", Password: testPassword, FullName: "Ada Lovelace",
	}, "Poro", "")
	require.NoError(t, err)
	require.Equal(t, "ada@poro.app", *res.User.Email)
	require.Equal(t, "Bearer", res.TokenType)
	for _, user := range users.byID {
		require.NotNil(t, user.PasswordHash)
		match, err := argon2id.ComparePasswordAndHash(testPassword, *user.PasswordHash)
		require.NoError(t, err)
		require.True(t, match)
	}

	_, err = svc.RegisterEmail(context.Background(), dto.RegisterEmailRequest{
		Email: "ada@poro.app", Password: testPassword, FullName: "Ada Lovelace",
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
		Role: model.RolePersonal, Status: model.StatusActive, CreatedAt: time.Now().UTC(),
	}
	svc := newAuthService(t, users, &fakeRefresh{}, &fakeOTP{}, &fakeTokens{})
	res, err := svc.LoginEmail(context.Background(), dto.LoginEmailRequest{Email: email, Password: testPassword}, "", "10.0.0.8")
	require.NoError(t, err)
	require.Equal(t, id.String(), res.User.ID)

	_, err = svc.LoginEmail(context.Background(), dto.LoginEmailRequest{Email: email, Password: "wrong-password"}, "", "")
	require.ErrorIs(t, err, ErrInvalidCredentials)

	_, err = svc.LoginEmail(context.Background(), dto.LoginEmailRequest{Email: "missing@poro.app", Password: testPassword}, "", "")
	require.ErrorIs(t, err, ErrInvalidCredentials)

	users.byID[id].Status = model.StatusDeleted
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

func TestRefreshRotatesAndCopiesClient(t *testing.T) {
	id := mustID(t)
	users := newFakeUsers()
	users.byID[id] = &model.User{ID: id, Role: model.RoleCreator, Status: model.StatusActive, CreatedAt: time.Now().UTC()}
	ua, ip := "Poro", "192.0.2.10"
	oldID := mustID(t)
	refresh := &fakeRefresh{items: []*model.RefreshToken{{
		ID: oldID, UserID: id, TokenHash: HashToken("old-refresh"),
		ExpiresAt: time.Now().UTC().Add(time.Hour), UserAgent: &ua, IPAddress: &ip,
	}}}
	svc := newAuthService(t, users, refresh, &fakeOTP{}, &fakeTokens{refresh: "new-refresh"})
	res, err := svc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "old-refresh"})
	require.NoError(t, err)
	require.Equal(t, "access-token", res.AccessToken)
	require.Equal(t, "new-refresh", res.RefreshToken)
	require.Equal(t, 900, res.ExpiresIn)
	require.NotNil(t, refresh.items[0].RevokedAt)
	require.Equal(t, ua, *refresh.items[1].UserAgent)
	require.Equal(t, ip, *refresh.items[1].IPAddress)
	require.Equal(t, id, refresh.items[1].UserID)
}

func TestRefreshErrors(t *testing.T) {
	id := mustID(t)
	user := &model.User{ID: id, Role: model.RolePersonal, Status: model.StatusActive, CreatedAt: time.Now().UTC()}
	valid := func() (*fakeUsers, *fakeRefresh) {
		users := newFakeUsers()
		users.byID[id] = user
		refresh := &fakeRefresh{items: []*model.RefreshToken{{
			ID: mustID(t), UserID: id, TokenHash: HashToken("old-refresh"),
			ExpiresAt: time.Now().UTC().Add(time.Hour),
		}}}
		return users, refresh
	}

	t.Run("unknown", func(t *testing.T) {
		svc := newAuthService(t, newFakeUsers(), &fakeRefresh{}, &fakeOTP{}, &fakeTokens{})
		_, err := svc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "missing"})
		require.ErrorIs(t, err, ErrRefreshTokenInvalid)
	})

	t.Run("store error", func(t *testing.T) {
		_, refresh := valid()
		refresh.getErr = errors.New("db")
		svc := newAuthService(t, newFakeUsers(), refresh, &fakeOTP{}, &fakeTokens{})
		_, err := svc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "old-refresh"})
		require.Error(t, err)
		require.NotErrorIs(t, err, ErrRefreshTokenInvalid)
	})

	t.Run("revoked", func(t *testing.T) {
		_, refresh := valid()
		now := time.Now().UTC()
		refresh.items[0].RevokedAt = &now
		svc := newAuthService(t, newFakeUsers(), refresh, &fakeOTP{}, &fakeTokens{})
		_, err := svc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "old-refresh"})
		require.ErrorIs(t, err, ErrRefreshTokenInvalid)
	})

	t.Run("expired", func(t *testing.T) {
		_, refresh := valid()
		refresh.items[0].ExpiresAt = time.Now().UTC().Add(-time.Minute)
		svc := newAuthService(t, newFakeUsers(), refresh, &fakeOTP{}, &fakeTokens{})
		_, err := svc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "old-refresh"})
		require.ErrorIs(t, err, ErrRefreshTokenInvalid)
	})

	t.Run("user missing", func(t *testing.T) {
		_, refresh := valid()
		svc := newAuthService(t, newFakeUsers(), refresh, &fakeOTP{}, &fakeTokens{})
		_, err := svc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "old-refresh"})
		require.ErrorIs(t, err, ErrRefreshTokenInvalid)
	})

	t.Run("user lookup failed", func(t *testing.T) {
		users, refresh := valid()
		users.getByIDErr = errors.New("db")
		svc := newAuthService(t, users, refresh, &fakeOTP{}, &fakeTokens{})
		_, err := svc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "old-refresh"})
		require.Error(t, err)
	})

	t.Run("blocked", func(t *testing.T) {
		users, refresh := valid()
		users.byID[id].Status = model.StatusSuspended
		svc := newAuthService(t, users, refresh, &fakeOTP{}, &fakeTokens{})
		_, err := svc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "old-refresh"})
		require.ErrorIs(t, err, ErrAccountBlocked)
	})

	t.Run("access failed", func(t *testing.T) {
		users, refresh := valid()
		svc := newAuthService(t, users, refresh, &fakeOTP{}, &fakeTokens{failAccess: true})
		_, err := svc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "old-refresh"})
		require.Error(t, err)
	})

	t.Run("refresh failed", func(t *testing.T) {
		users, refresh := valid()
		svc := newAuthService(t, users, refresh, &fakeOTP{}, &fakeTokens{failRefresh: true})
		_, err := svc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "old-refresh"})
		require.Error(t, err)
	})

	t.Run("store create failed", func(t *testing.T) {
		users, refresh := valid()
		refresh.createErr = errors.New("db")
		svc := newAuthService(t, users, refresh, &fakeOTP{}, &fakeTokens{})
		_, err := svc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "old-refresh"})
		require.Error(t, err)
	})

	t.Run("revoke failed", func(t *testing.T) {
		users, refresh := valid()
		refresh.revokeErr = errors.New("db")
		svc := newAuthService(t, users, refresh, &fakeOTP{}, &fakeTokens{})
		_, err := svc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "old-refresh"})
		require.Error(t, err)
	})
}

func TestLogout(t *testing.T) {
	id := mustID(t)
	refresh := &fakeRefresh{items: []*model.RefreshToken{{
		ID: mustID(t), UserID: id, TokenHash: HashToken("refresh-token"),
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}}}
	tokens := &fakeTokens{userID: id}
	svc := newAuthService(t, newFakeUsers(), refresh, &fakeOTP{}, tokens)
	require.NoError(t, svc.Logout(context.Background(), "access-token", "refresh-token"))
	require.Equal(t, []string{"access-token"}, tokens.blacklisted)
	require.NotNil(t, refresh.items[0].RevokedAt)

	already := time.Now().UTC()
	refresh.items[0].RevokedAt = &already
	require.NoError(t, svc.Logout(context.Background(), "access-token", "refresh-token"))

	require.NoError(t, svc.Logout(context.Background(), "access-token", ""))

	refresh.getErr = repository.ErrRefreshTokenNotFound
	refresh.items = nil
	require.NoError(t, svc.Logout(context.Background(), "access-token", "missing"))

	tokens.failValidate = true
	require.Error(t, svc.Logout(context.Background(), "access-token", "refresh-token"))
	tokens.failValidate = false

	tokens.failBlacklist = true
	require.Error(t, svc.Logout(context.Background(), "access-token", ""))
	tokens.failBlacklist = false

	refresh.getErr = errors.New("db")
	require.Error(t, svc.Logout(context.Background(), "access-token", "refresh-token"))
	refresh.getErr = nil

	refresh.items = []*model.RefreshToken{{
		ID: mustID(t), UserID: id, TokenHash: HashToken("refresh-token"),
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}}
	refresh.revokeErr = errors.New("db")
	require.Error(t, svc.Logout(context.Background(), "access-token", "refresh-token"))
}

func TestMe(t *testing.T) {
	id := mustID(t)
	phone := "+2250707070707"
	users := newFakeUsers()
	created := time.Date(2026, 9, 27, 11, 0, 0, 0, time.UTC)
	users.byID[id] = &model.User{
		ID: id, Phone: &phone, Role: model.RolePersonal, Status: model.StatusActive,
		Language: "fr", CreatedAt: created,
	}
	svc := newAuthService(t, users, &fakeRefresh{}, &fakeOTP{}, &fakeTokens{})
	res, err := svc.Me(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, id.String(), res.ID)
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
	users := newFakeUsers()
	svc := newAuthService(t, users, &fakeRefresh{createErr: errors.New("db")}, &fakeOTP{}, &fakeTokens{})
	_, err := svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: phone, Code: "123456"}, "", "")
	require.Error(t, err)

	users = newFakeUsers()
	users.lastLoginErr = errors.New("db")
	svc = newAuthService(t, users, &fakeRefresh{}, &fakeOTP{}, &fakeTokens{})
	_, err = svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: phone, Code: "123456"}, "", "")
	require.Error(t, err)

	svc = newAuthService(t, newFakeUsers(), &fakeRefresh{}, &fakeOTP{}, &fakeTokens{failRefresh: true})
	_, err = svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: phone, Code: "123456"}, "", "")
	require.Error(t, err)
}

func newAuthService(t *testing.T, users *fakeUsers, refresh *fakeRefresh, otp OTPService, tokens TokenService) AuthService {
	t.Helper()
	svc, err := NewAuthService(
		&config.Config{JWTAccessTTL: "15m", JWTRefreshTTL: "720h"},
		users, refresh, otp, tokens, zap.NewNop(),
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
		passwordHash, passwordErr = argon2id.CreateHash(testPassword, argon2id.DefaultParams)
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
	if user.Phone != nil {
		for _, existing := range f.byID {
			if existing.Phone != nil && *existing.Phone == *user.Phone {
				return repository.ErrUserAlreadyExists
			}
		}
	}
	if user.Email != nil {
		for _, existing := range f.byID {
			if existing.Email != nil && *existing.Email == *user.Email {
				return repository.ErrUserAlreadyExists
			}
		}
	}
	if user.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		user.ID = id
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

func (f *fakeUsers) ExistsByPhone(_ context.Context, phone string) (bool, error) {
	_, err := f.GetByPhone(context.Background(), phone)
	if errors.Is(err, repository.ErrUserNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (f *fakeUsers) ExistsByEmail(_ context.Context, email string) (bool, error) {
	_, err := f.GetByEmail(context.Background(), email)
	if errors.Is(err, repository.ErrUserNotFound) {
		return false, nil
	}
	return err == nil, err
}

type fakeRefresh struct {
	items     []*model.RefreshToken
	createErr error
	getErr    error
	revokeErr error
}

func (f *fakeRefresh) Create(_ context.Context, token *model.RefreshToken) error {
	if f.createErr != nil {
		return f.createErr
	}
	if token.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		token.ID = id
	}
	f.items = append(f.items, token)
	return nil
}

func (f *fakeRefresh) GetByHash(_ context.Context, hash string) (*model.RefreshToken, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	for _, token := range f.items {
		if token.TokenHash == hash {
			return token, nil
		}
	}
	return nil, repository.ErrRefreshTokenNotFound
}

func (f *fakeRefresh) Revoke(_ context.Context, id uuid.UUID) error {
	if f.revokeErr != nil {
		return f.revokeErr
	}
	for _, token := range f.items {
		if token.ID == id {
			now := time.Now().UTC()
			token.RevokedAt = &now
			return nil
		}
	}
	return repository.ErrRefreshTokenNotFound
}

func (f *fakeRefresh) RevokeAllForUser(context.Context, uuid.UUID) error { return nil }

func (f *fakeRefresh) DeleteExpired(context.Context) (int64, error) { return 0, nil }

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
	userID        uuid.UUID
	access        string
	refresh       string
	failAccess    bool
	failRefresh   bool
	failValidate  bool
	failBlacklist bool
	blacklisted   []string
}

func (f *fakeTokens) GenerateAccessToken(uuid.UUID, string) (string, error) {
	if f.failAccess {
		return "", errors.New("sign")
	}
	if f.access == "" {
		return "access-token", nil
	}
	return f.access, nil
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
	if f.failValidate {
		return nil, ErrInvalidToken
	}
	return &TokenClaims{UserID: f.userID, Role: string(model.RolePersonal), Exp: time.Now().UTC().Add(time.Minute)}, nil
}

func (f *fakeTokens) HashToken(token string) string { return HashToken(token) }

func (f *fakeTokens) BlacklistToken(_ context.Context, token string, _ time.Duration) error {
	if f.failBlacklist {
		return errors.New("redis")
	}
	f.blacklisted = append(f.blacklisted, token)
	return nil
}

func (f *fakeTokens) IsBlacklisted(context.Context, string) (bool, error) { return false, nil }
