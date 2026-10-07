package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgx5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"

	"github.com/poro/social/internal/config"
)

// RunMigrations applies every up migration in the migrations directory.
func RunMigrations(ctx context.Context, cfg *config.Config, log *zap.Logger) error {
	dir, err := findMigrationsDir()
	if err != nil {
		return err
	}

	db, err := sql.Open("pgx", cfg.PostgresDSN())
	if err != nil {
		return fmt.Errorf("open postgres for migrations: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return fmt.Errorf("ping postgres for migrations: %w", err)
	}

	driver, err := pgx5.WithInstance(db, &pgx5.Config{MultiStatementEnabled: true})
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("init migration driver: %w", err)
	}

	source, err := iofs.New(os.DirFS(dir), ".")
	if err != nil {
		_ = driver.Close()
		return fmt.Errorf("open migrations in %s: %w", dir, err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "pgx5", driver)
	if err != nil {
		_ = driver.Close()
		return fmt.Errorf("create migrator: %w", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil || dbErr != nil {
			log.Warn("close migrator", zap.NamedError("source", srcErr), zap.NamedError("database", dbErr))
		}
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}

	version, dirty, verr := m.Version()
	if verr != nil && !errors.Is(verr, migrate.ErrNilVersion) {
		return fmt.Errorf("read migration version: %w", verr)
	}
	log.Info("migrations ready", zap.String("dir", dir), zap.Uint("version", version), zap.Bool("dirty", dirty))
	return nil
}

func findMigrationsDir() (string, error) {
	if dir := os.Getenv("MIGRATIONS_PATH"); dir != "" {
		return dir, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	dir := cwd
	for {
		candidate := filepath.Join(dir, "migrations", "000001_create_social.up.sql")
		if _, statErr := os.Stat(candidate); statErr == nil {
			return filepath.Join(dir, "migrations"), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("migrations directory not found from %s", cwd)
		}
		dir = parent
	}
}
