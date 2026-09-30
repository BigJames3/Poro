package repository_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.uber.org/zap"

	"github.com/poro/auth/internal/config"
	"github.com/poro/auth/internal/database"
	"github.com/poro/auth/internal/model"
	"github.com/poro/auth/internal/repository"
)

var (
	testPool *pgxpool.Pool
	testCfg  *config.Config
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("poro_auth"),
		postgres.WithUsername("poro"),
		postgres.WithPassword("poro_dev_password"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start postgres: %v\n", err)
		os.Exit(1)
	}

	host, err := container.Host(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "postgres host: %v\n", err)
		_ = container.Terminate(ctx)
		os.Exit(1)
	}
	if host == "localhost" {
		host = "127.0.0.1"
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "postgres port: %v\n", err)
		_ = container.Terminate(ctx)
		os.Exit(1)
	}

	testCfg = &config.Config{
		PostgresHost:     host,
		PostgresPort:     port.Port(),
		PostgresUser:     "poro",
		PostgresPassword: "poro_dev_password",
		PostgresDB:       "poro_auth",
		PostgresSSLMode:  "disable",
	}
	log := zap.NewNop()
	if err := database.RunMigrations(ctx, testCfg, log); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		_ = container.Terminate(ctx)
		os.Exit(1)
	}
	testPool, err = database.NewPostgresPool(ctx, testCfg, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pool: %v\n", err)
		_ = container.Terminate(ctx)
		os.Exit(1)
	}

	code := m.Run()
	testPool.Close()
	_ = container.Terminate(context.Background())
	os.Exit(code)
}

func TestUserRepositoryCreateAndGet(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewUserRepository(testPool)
	phone := uniquePhone()
	email := uniqueEmail()
	country := "CI"
	user := &model.User{Phone: &phone, Email: &email, CountryCode: &country}

	require.NoError(t, repo.Create(ctx, user))
	require.NotEqual(t, uuid.Nil, user.ID)
	require.Equal(t, []model.UserRole{model.RolePersonal}, user.Roles)
	require.Equal(t, model.StatusPending, user.Status)
	require.Equal(t, "fr", user.Language)

	byID, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, phone, *byID.Phone)
	require.Equal(t, email, *byID.Email)
	require.Equal(t, "CI", *byID.CountryCode)
	require.Equal(t, []model.UserRole{model.RolePersonal}, byID.Roles)

	byPhone, err := repo.GetByPhone(ctx, phone)
	require.NoError(t, err)
	require.Equal(t, user.ID, byPhone.ID)

	byEmail, err := repo.GetByEmail(ctx, email)
	require.NoError(t, err)
	require.Equal(t, user.ID, byEmail.ID)

	phoneExists, err := repo.ExistsByPhone(ctx, phone)
	require.NoError(t, err)
	require.True(t, phoneExists)
	emailExists, err := repo.ExistsByEmail(ctx, email)
	require.NoError(t, err)
	require.True(t, emailExists)
}

func TestUserRepositoryMultipleRoles(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewUserRepository(testPool)
	roles := []model.UserRole{model.RolePersonal, model.RoleCreator, model.RoleBusiness, model.RoleEnterprise}
	user := &model.User{Phone: ptr(uniquePhone()), Roles: roles}
	require.NoError(t, repo.Create(ctx, user))

	stored, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.ElementsMatch(t, roles, stored.Roles)

	stored.Language = "en"
	require.NoError(t, repo.Update(ctx, stored))
	again, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.ElementsMatch(t, roles, again.Roles, "Update leaves roles untouched")
}

func TestUserRepositoryRejectsUnknownRoleAtomically(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewUserRepository(testPool)
	phone := uniquePhone()
	err := repo.Create(ctx, &model.User{Phone: &phone, Roles: []model.UserRole{"SUPERUSER"}})
	require.Error(t, err)

	_, err = repo.GetByPhone(ctx, phone)
	require.ErrorIs(t, err, repository.ErrUserNotFound, "a failed role insert rolls the user back")
}

func TestUserRepositoryRejectsDenormalizedEmail(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewUserRepository(testPool)
	email := "Mixed.Case@Poro.Test"
	err := repo.Create(ctx, &model.User{Email: &email})
	require.Error(t, err, "the database refuses emails that are not lowercased and trimmed")
}

func TestUserRepositoryDuplicateAndSoftDelete(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewUserRepository(testPool)
	phone := uniquePhone()
	first := &model.User{Phone: &phone}
	require.NoError(t, repo.Create(ctx, first))

	err := repo.Create(ctx, &model.User{Phone: &phone})
	require.ErrorIs(t, err, repository.ErrUserAlreadyExists)

	require.NoError(t, repo.SoftDelete(ctx, first.ID))
	_, err = repo.GetByPhone(ctx, phone)
	require.ErrorIs(t, err, repository.ErrUserNotFound)
	exists, err := repo.ExistsByPhone(ctx, phone)
	require.NoError(t, err)
	require.False(t, exists)

	second := &model.User{Phone: &phone, Status: model.StatusActive}
	require.NoError(t, repo.Create(ctx, second))
	require.NotEqual(t, first.ID, second.ID)
}

func TestUserRepositoryUpdateAndLastLogin(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewUserRepository(testPool)
	user := &model.User{Phone: ptr(uniquePhone())}
	require.NoError(t, repo.Create(ctx, user))

	user.Language = "en"
	user.Status = model.StatusActive
	require.NoError(t, repo.Update(ctx, user))

	stored, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, "en", stored.Language)
	require.Equal(t, model.StatusActive, stored.Status)

	require.NoError(t, repo.UpdateLastLogin(ctx, user.ID))
	stored, err = repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.LastLoginAt)
	require.WithinDuration(t, time.Now(), *stored.LastLoginAt, 5*time.Second)

	missing := uuid.Must(uuid.NewV7())
	require.ErrorIs(t, repo.UpdateLastLogin(ctx, missing), repository.ErrUserNotFound)
	_, err = repo.GetByID(ctx, missing)
	require.ErrorIs(t, err, repository.ErrUserNotFound)
}

func ptr(v string) *string { return &v }

func uniquePhone() string {
	id := strings.ReplaceAll(uuid.Must(uuid.NewV7()).String(), "-", "")
	return "+225" + id[len(id)-10:]
}

func uniqueEmail() string {
	id := strings.ReplaceAll(uuid.Must(uuid.NewV7()).String(), "-", "")
	return id[len(id)-16:] + "@poro.test"
}
