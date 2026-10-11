package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/auth/internal/dto"
	"github.com/poro/auth/internal/model"
	"github.com/poro/auth/internal/repository"
)

const testGrace = 720 * time.Hour

type accountFixture struct {
	svc     *accountService
	users   *fakeUsers
	refresh *fakeRefresh
	otp     *fakeOTP
	tokens  *fakeTokens
	now     time.Time
}

func newAccountFixture(t *testing.T) *accountFixture {
	t.Helper()
	f := &accountFixture{
		users:   newFakeUsers(),
		refresh: newFakeRefresh(),
		otp:     &fakeOTP{},
		tokens:  &fakeTokens{},
		now:     time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC),
	}
	svc, err := NewAccountService(f.users, f.refresh, f.otp, f.tokens, testGrace, zap.NewNop())
	require.NoError(t, err)
	f.svc = svc.(*accountService)
	f.svc.now = func() time.Time { return f.now }
	return f
}

func (f *accountFixture) addUser(t *testing.T, user *model.User) *model.User {
	t.Helper()
	if user.Status == "" {
		user.Status = model.StatusActive
	}
	require.NoError(t, f.users.Create(context.Background(), user))
	return user
}

func claimsFor(user *model.User, session uuid.UUID) TokenClaims {
	return TokenClaims{UserID: user.ID, SessionID: session, TokenID: "jti-current", Exp: time.Now().Add(time.Minute)}
}

func TestNewAccountServiceRejectsNonPositiveGrace(t *testing.T) {
	_, err := NewAccountService(newFakeUsers(), newFakeRefresh(), &fakeOTP{}, &fakeTokens{}, 0, zap.NewNop())
	require.Error(t, err)
}

func TestSessionsListsLiveDevicesAndMarksTheCurrentOne(t *testing.T) {
	f := newAccountFixture(t)
	user := f.addUser(t, &model.User{Phone: ptrTo("+2250707070707")})
	other := f.addUser(t, &model.User{Phone: ptrTo("+2250707070708")})

	phone := f.refresh.seed(t, user.ID, "phone")
	phone.DeviceName = ptrTo("Tecno Spark 20")
	phone.IPAddress = ptrTo("41.202.10.1")
	browser := f.refresh.seed(t, user.ID, "browser")
	browser.UserAgent = ptrTo("Mozilla/5.0 " + longText(150))
	bare := f.refresh.seed(t, user.ID, "bare")
	ended := f.refresh.seed(t, user.ID, "ended")
	revoked := time.Now()
	ended.RevokedAt = &revoked
	f.refresh.seed(t, other.ID, "someone-else")

	sessions, err := f.svc.Sessions(context.Background(), claimsFor(user, phone.FamilyID))
	require.NoError(t, err)
	require.Len(t, sessions, 3)

	byID := map[string]dto.SessionResponse{}
	for _, s := range sessions {
		byID[s.ID] = s
	}
	require.True(t, byID[phone.FamilyID.String()].Current)
	require.Equal(t, "Tecno Spark 20", *byID[phone.FamilyID.String()].Device)
	require.Equal(t, "41.202.10.1", *byID[phone.FamilyID.String()].IPAddress)
	require.False(t, byID[browser.FamilyID.String()].Current)
	require.Len(t, []rune(*byID[browser.FamilyID.String()].Device), maxDeviceLabelLen)
	require.Nil(t, byID[bare.FamilyID.String()].Device)
	require.NotContains(t, byID, ended.FamilyID.String())

	f.refresh.getErr = errors.New("db down")
	_, err = f.svc.Sessions(context.Background(), claimsFor(user, phone.FamilyID))
	require.Error(t, err)
}

func TestRevokeSession(t *testing.T) {
	f := newAccountFixture(t)
	user := f.addUser(t, &model.User{Phone: ptrTo("+2250707070707")})
	current := f.refresh.seed(t, user.ID, "current")
	other := f.refresh.seed(t, user.ID, "other")
	stranger := f.refresh.seed(t, uuid.Must(uuid.NewV7()), "stranger")
	claims := claimsFor(user, current.FamilyID)
	ctx := context.Background()

	require.NoError(t, f.svc.RevokeSession(ctx, claims, other.FamilyID))
	require.NotNil(t, other.RevokedAt)
	require.Nil(t, current.RevokedAt)
	require.Empty(t, f.tokens.blacklisted, "another device: the caller keeps its access token")

	require.ErrorIs(t, f.svc.RevokeSession(ctx, claims, other.FamilyID), ErrSessionNotFound, "already ended")
	require.ErrorIs(t, f.svc.RevokeSession(ctx, claims, stranger.FamilyID), ErrSessionNotFound, "not the caller's")
	require.Nil(t, stranger.RevokedAt)

	require.NoError(t, f.svc.RevokeSession(ctx, claims, current.FamilyID))
	require.NotNil(t, current.RevokedAt)
	require.Equal(t, []string{"jti-current"}, f.tokens.blacklisted, "the current device is logged out")

	f.tokens.failBlacklist = true
	require.Error(t, f.svc.RevokeSession(ctx, claims, current.FamilyID))
	f.tokens.failBlacklist = false
	f.refresh.revokeErr = errors.New("db down")
	require.Error(t, f.svc.RevokeSession(ctx, claims, other.FamilyID))
}

func TestRevokeOtherSessionsKeepsTheCurrentDevice(t *testing.T) {
	f := newAccountFixture(t)
	user := f.addUser(t, &model.User{Phone: ptrTo("+2250707070707")})
	current := f.refresh.seed(t, user.ID, "current")
	first := f.refresh.seed(t, user.ID, "first")
	second := f.refresh.seed(t, user.ID, "second")

	res, err := f.svc.RevokeOtherSessions(context.Background(), claimsFor(user, current.FamilyID))
	require.NoError(t, err)
	require.Equal(t, int64(2), res.Revoked)
	require.Nil(t, current.RevokedAt)
	require.NotNil(t, first.RevokedAt)
	require.NotNil(t, second.RevokedAt)

	f.refresh.revokeErr = errors.New("db down")
	_, err = f.svc.RevokeOtherSessions(context.Background(), claimsFor(user, current.FamilyID))
	require.Error(t, err)
}

func TestDeleteEmailAccountNeedsItsPassword(t *testing.T) {
	f := newAccountFixture(t)
	hash := hashPassword(t)
	user := f.addUser(t, &model.User{Email: ptrTo("awa@poro.test"), PasswordHash: &hash})
	current := f.refresh.seed(t, user.ID, "current")
	other := f.refresh.seed(t, user.ID, "other")
	claims := claimsFor(user, current.FamilyID)
	ctx := context.Background()

	_, err := f.svc.DeleteAccount(ctx, claims, dto.DeleteAccountRequest{})
	require.ErrorIs(t, err, ErrReauthRequired)
	_, err = f.svc.DeleteAccount(ctx, claims, dto.DeleteAccountRequest{OTPCode: "123456"})
	require.ErrorIs(t, err, ErrReauthRequired, "an email account proves itself with its password")
	_, err = f.svc.DeleteAccount(ctx, claims, dto.DeleteAccountRequest{Password: "wrong-password"})
	require.ErrorIs(t, err, ErrInvalidCredentials)
	require.Nil(t, user.DeletionScheduledAt)
	require.Nil(t, current.RevokedAt)

	res, err := f.svc.DeleteAccount(ctx, claims, dto.DeleteAccountRequest{Password: testPassword})
	require.NoError(t, err)
	want := f.now.Add(testGrace)
	require.Equal(t, want.Format(time.RFC3339), res.DeletionScheduledAt)
	require.Equal(t, want, *user.DeletionScheduledAt)
	require.NotNil(t, current.RevokedAt)
	require.NotNil(t, other.RevokedAt, "every device is signed out")
	require.Equal(t, []string{"jti-current"}, f.tokens.blacklisted)

	f.now = f.now.Add(48 * time.Hour)
	again, err := f.svc.DeleteAccount(ctx, claims, dto.DeleteAccountRequest{Password: testPassword})
	require.NoError(t, err)
	require.Equal(t, res.DeletionScheduledAt, again.DeletionScheduledAt, "asking again keeps the first date")
}

func TestDeletePhoneAccountNeedsAFreshCode(t *testing.T) {
	f := newAccountFixture(t)
	user := f.addUser(t, &model.User{Phone: ptrTo("+2250707070707")})
	claims := claimsFor(user, uuid.Must(uuid.NewV7()))
	ctx := context.Background()

	_, err := f.svc.DeleteAccount(ctx, claims, dto.DeleteAccountRequest{Password: testPassword})
	require.ErrorIs(t, err, ErrReauthRequired, "a phone account has no password")

	f.otp.err = ErrOTPInvalid
	_, err = f.svc.DeleteAccount(ctx, claims, dto.DeleteAccountRequest{OTPCode: "000000"})
	require.ErrorIs(t, err, ErrOTPInvalid)
	require.Nil(t, user.DeletionScheduledAt)

	f.otp.err = nil
	_, err = f.svc.DeleteAccount(ctx, claims, dto.DeleteAccountRequest{OTPCode: "123456"})
	require.NoError(t, err)
	require.NotNil(t, user.DeletionScheduledAt)
}

func TestDeleteAccountRefusals(t *testing.T) {
	f := newAccountFixture(t)
	ctx := context.Background()
	stranger := TokenClaims{UserID: uuid.Must(uuid.NewV7()), SessionID: uuid.Must(uuid.NewV7())}
	_, err := f.svc.DeleteAccount(ctx, stranger, dto.DeleteAccountRequest{Password: testPassword})
	require.ErrorIs(t, err, ErrAccountNotFound)

	suspended := f.addUser(t, &model.User{Phone: ptrTo("+2250707070701"), Status: model.StatusSuspended})
	_, err = f.svc.DeleteAccount(ctx, claimsFor(suspended, uuid.Nil), dto.DeleteAccountRequest{OTPCode: "123456"})
	require.ErrorIs(t, err, ErrAccountBlocked)

	nothing := f.addUser(t, &model.User{})
	_, err = f.svc.DeleteAccount(ctx, claimsFor(nothing, uuid.Nil), dto.DeleteAccountRequest{OTPCode: "123456"})
	require.ErrorIs(t, err, ErrReauthRequired, "an account without identifier cannot prove anything")

	hash := "not-an-argon2-hash"
	broken := f.addUser(t, &model.User{Email: ptrTo("broken@poro.test"), PasswordHash: &hash})
	_, err = f.svc.DeleteAccount(ctx, claimsFor(broken, uuid.Nil), dto.DeleteAccountRequest{Password: testPassword})
	require.Error(t, err)

	user := f.addUser(t, &model.User{Phone: ptrTo("+2250707070702")})
	claims := claimsFor(user, uuid.Nil)
	f.users.getByIDErr = errors.New("db down")
	_, err = f.svc.DeleteAccount(ctx, claims, dto.DeleteAccountRequest{OTPCode: "123456"})
	require.Error(t, err)
	f.users.getByIDErr = nil

	f.users.scheduleErr = errors.New("db down")
	_, err = f.svc.DeleteAccount(ctx, claims, dto.DeleteAccountRequest{OTPCode: "123456"})
	require.Error(t, err)
	f.users.scheduleErr = nil

	f.refresh.revokeErr = errors.New("db down")
	_, err = f.svc.DeleteAccount(ctx, claims, dto.DeleteAccountRequest{OTPCode: "123456"})
	require.Error(t, err)
	f.refresh.revokeErr = nil

	f.tokens.failBlacklist = true
	_, err = f.svc.DeleteAccount(ctx, claims, dto.DeleteAccountRequest{OTPCode: "123456"})
	require.Error(t, err)
}

func TestCancelDeletion(t *testing.T) {
	f := newAccountFixture(t)
	user := f.addUser(t, &model.User{Phone: ptrTo("+2250707070707")})
	claims := claimsFor(user, uuid.Nil)
	ctx := context.Background()
	scheduled := f.now.Add(testGrace)
	user.DeletionScheduledAt = &scheduled

	res, err := f.svc.CancelDeletion(ctx, claims)
	require.NoError(t, err)
	require.Nil(t, res.DeletionScheduledAt)
	require.Nil(t, user.DeletionScheduledAt)

	res, err = f.svc.CancelDeletion(ctx, claims)
	require.NoError(t, err, "nothing to cancel is not an error")
	require.Equal(t, user.ID.String(), res.ID)

	_, err = f.svc.CancelDeletion(ctx, TokenClaims{UserID: uuid.Must(uuid.NewV7())})
	require.ErrorIs(t, err, ErrAccountNotFound)

	f.users.getByIDErr = errors.New("db down")
	_, err = f.svc.CancelDeletion(ctx, claims)
	require.Error(t, err)
	f.users.cancelErr = errors.New("db down")
	_, err = f.svc.CancelDeletion(ctx, claims)
	require.Error(t, err)
}

func TestSignInCancelsAPendingDeletion(t *testing.T) {
	users := newFakeUsers()
	refresh := newFakeRefresh()
	svc := newAuthService(t, users, refresh, &fakeOTP{}, &fakeTokens{})
	phone := "+2250707070707"
	scheduled := time.Now().Add(testGrace)
	user := &model.User{Phone: &phone, Status: model.StatusActive, DeletionScheduledAt: &scheduled}
	require.NoError(t, users.Create(context.Background(), user))

	res, err := svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: phone, Code: "123456", DeviceName: "  Itel\tA70 \x07 "}, "PoroAndroid/1.0", "41.202.10.1")
	require.NoError(t, err)
	require.True(t, res.DeletionCancelled)
	require.Nil(t, res.User.DeletionScheduledAt)
	require.Nil(t, users.byID[user.ID].DeletionScheduledAt)
	require.Equal(t, "Itel A70", *refresh.items[0].DeviceName, "the device name is cleaned")

	res, err = svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: phone, Code: "123456"}, "", "")
	require.NoError(t, err)
	require.False(t, res.DeletionCancelled)
	require.Nil(t, refresh.items[1].DeviceName)
	require.Nil(t, refresh.items[1].UserAgent)

	users.byID[user.ID].DeletionScheduledAt = &scheduled
	users.cancelErr = errors.New("db down")
	_, err = svc.VerifyOTP(context.Background(), dto.VerifyOTPRequest{Phone: phone, Code: "123456"}, "", "")
	require.Error(t, err)
}

func TestRefreshKeepsTheDeviceName(t *testing.T) {
	users := newFakeUsers()
	refresh := newFakeRefresh()
	tokens := &fakeTokens{refresh: "next"}
	svc := newAuthService(t, users, refresh, &fakeOTP{}, tokens)
	user := &model.User{Phone: ptrTo("+2250707070707"), Status: model.StatusActive}
	require.NoError(t, users.Create(context.Background(), user))
	first := refresh.seed(t, user.ID, "first")
	first.DeviceName = ptrTo("Tecno Spark 20")

	_, err := svc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "first"})
	require.NoError(t, err)
	require.Equal(t, "Tecno Spark 20", *refresh.items[1].DeviceName)
}

func TestToUserResponseShowsTheDeletionDate(t *testing.T) {
	at := time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)
	res := toUserResponse(&model.User{DeletionScheduledAt: &at})
	require.Equal(t, "2026-11-11T09:00:00Z", *res.DeletionScheduledAt)
	require.Nil(t, toUserResponse(&model.User{}).DeletionScheduledAt)
}

func TestCleanDeviceName(t *testing.T) {
	require.Equal(t, "Samsung Galaxy A14", cleanDeviceName("  Samsung\n Galaxy\u200b A14  "))
	require.Empty(t, cleanDeviceName(" \t "))
	require.Len(t, []rune(cleanDeviceName(longText(80))), 60)
}

func TestAccountPurgeJob(t *testing.T) {
	purger := &fakePurger{batches: [][]uuid.UUID{ids(purgeBatchSize), ids(3)}}
	codes := &fakeExpiredCodes{deleted: 4}
	job, err := NewAccountPurgeJob(purger, codes, time.Hour, zap.NewNop())
	require.NoError(t, err)

	erased, err := job.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, purgeBatchSize+3, erased, "a full batch is followed by another")
	require.Equal(t, 2, purger.calls)

	purger.err = errors.New("db down")
	_, err = job.RunOnce(context.Background())
	require.Error(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	job.Run(ctx)
	require.Equal(t, 1, codes.calls, "a pass also cleans expired email codes")
	require.WithinDuration(t, time.Now().Add(-expiredCodeRetention), codes.before, time.Minute)

	codes.err = errors.New("db down")
	job.Run(ctx)
	require.Equal(t, 2, codes.calls, "a failed cleanup does not stop the job")

	_, err = NewAccountPurgeJob(purger, codes, 0, zap.NewNop())
	require.Error(t, err)
}

type fakePurger struct {
	batches [][]uuid.UUID
	calls   int
	err     error
}

func (f *fakePurger) PurgeDue(context.Context, time.Time, int) ([]uuid.UUID, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.calls++
	if len(f.batches) == 0 {
		return nil, nil
	}
	batch := f.batches[0]
	f.batches = f.batches[1:]
	return batch, nil
}

var _ repository.AccountPurger = (*fakePurger)(nil)

func ids(n int) []uuid.UUID {
	out := make([]uuid.UUID, n)
	for i := range out {
		out[i] = uuid.Must(uuid.NewV7())
	}
	return out
}

func ptrTo(value string) *string { return &value }

func longText(n int) string {
	runes := make([]rune, n)
	for i := range runes {
		runes[i] = 'é'
	}
	return string(runes)
}

type fakeExpiredCodes struct {
	deleted int64
	calls   int
	before  time.Time
	err     error
}

func (f *fakeExpiredCodes) DeleteExpired(_ context.Context, before time.Time) (int64, error) {
	f.calls++
	f.before = before
	if f.err != nil {
		return 0, f.err
	}
	return f.deleted, nil
}
