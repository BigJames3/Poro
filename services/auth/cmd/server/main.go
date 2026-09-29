package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/poro/auth/internal/config"
	"github.com/poro/auth/internal/database"
	"github.com/poro/auth/internal/dto"
	"github.com/poro/auth/internal/handler"
	"github.com/poro/auth/internal/middleware"
	"github.com/poro/auth/internal/repository"
	"github.com/poro/auth/internal/routes"
	"github.com/poro/auth/internal/service"
)

const version = "1.0.0"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "auth server: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	log, err := newLogger(cfg)
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	ctx := context.Background()
	pool := openPostgres(ctx, cfg, log)
	rdb := openRedis(ctx, cfg, log)
	defer closeStores(pool, rdb)

	if pool != nil {
		if err := database.RunMigrations(ctx, cfg, log); err != nil {
			log.Warn("postgres migrations failed", zap.Error(err))
		}
	}

	tokens, err := openTokens(cfg, rdb, log)
	if err != nil {
		return err
	}
	authSvc, err := openAuth(cfg, pool, tokens, log)
	if err != nil {
		return err
	}

	app := newApp(log)
	routes.Register(app, routes.Dependencies{
		Auth:    handler.NewAuthHandler(authSvc),
		Health:  handler.NewHealthHandler(pool, rdb, version),
		Tokens:  tokens,
		Limiter: openLimiter(cfg, rdb, log),
	})

	addr := ":" + cfg.AppPort
	errCh := make(chan error, 1)
	go func() { errCh <- app.Listen(addr) }()
	log.Info("auth service started", zap.String("port", cfg.AppPort), zap.String("env", cfg.AppEnv))

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("listen %s: %w", addr, err)
		}
		return nil
	case <-stop:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := app.ShutdownWithContext(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		log.Info("auth service stopped")
		return nil
	}
}

func openPostgres(ctx context.Context, cfg *config.Config, log *zap.Logger) *pgxpool.Pool {
	pool, err := database.NewPostgresPool(ctx, cfg, log)
	if err != nil {
		log.Warn("postgres unavailable", zap.Error(err))
		return nil
	}
	return pool
}

func openRedis(ctx context.Context, cfg *config.Config, log *zap.Logger) *redis.Client {
	rdb, err := database.NewRedisClient(ctx, cfg, log)
	if err != nil {
		log.Warn("redis unavailable", zap.Error(err))
		return nil
	}
	return rdb
}

func openLimiter(cfg *config.Config, rdb *redis.Client, log *zap.Logger) fiber.Storage {
	if rdb == nil {
		log.Warn("rate limiter using process memory")
		return nil
	}
	store, err := middleware.NewRedisStorage(cfg)
	if err != nil {
		log.Warn("rate limiter using process memory", zap.Error(err))
		return nil
	}
	return store
}

func openTokens(cfg *config.Config, rdb *redis.Client, log *zap.Logger) (service.TokenService, error) {
	tokens, err := service.NewTokenService(cfg, rdb)
	if err == nil {
		return tokens, nil
	}
	if !isKeyError(err) {
		return nil, fmt.Errorf("init token service: %w", err)
	}
	log.Error("jwt keys unavailable", zap.Error(err))
	return unavailableTokens{err: err}, nil
}

func openAuth(cfg *config.Config, pool *pgxpool.Pool, tokens service.TokenService, log *zap.Logger) (service.AuthService, error) {
	if pool == nil {
		return unavailableAuth{err: errors.New("postgres unavailable")}, nil
	}
	otp, err := service.NewOTPService(cfg, repository.NewOTPRepository(pool), log)
	if err != nil {
		return nil, fmt.Errorf("init otp service: %w", err)
	}
	authSvc, err := service.NewAuthService(
		cfg,
		repository.NewUserRepository(pool),
		repository.NewRefreshTokenRepository(pool),
		otp,
		tokens,
		log,
	)
	if err != nil {
		return nil, fmt.Errorf("init auth service: %w", err)
	}
	return authSvc, nil
}

func closeStores(pool *pgxpool.Pool, rdb *redis.Client) {
	if pool != nil {
		pool.Close()
	}
	if rdb != nil {
		_ = rdb.Close()
	}
}

func isKeyError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "private key") || strings.Contains(msg, "public key")
}

func newApp(log *zap.Logger) *fiber.App {
	return fiber.New(fiber.Config{
		AppName:               "poro-auth",
		DisableStartupMessage: true,
		ReadTimeout:           10 * time.Second,
		WriteTimeout:          10 * time.Second,
		IdleTimeout:           30 * time.Second,
		BodyLimit:             4 * 1024 * 1024,
		ErrorHandler:          errorHandler(log),
	})
}

func errorHandler(log *zap.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		code, msg := fiber.StatusInternalServerError, "internal server error"
		var fe *fiber.Error
		if errors.As(err, &fe) {
			code, msg = fe.Code, fe.Message
		}
		log.Error("request failed",
			zap.Int("status", code),
			zap.String("method", c.Method()),
			zap.String("path", c.Path()),
			zap.Error(err),
		)
		return c.Status(code).JSON(fiber.Map{
			"data": nil, "error": fiber.Map{"message": msg}, "meta": nil,
		})
	}
}

func newLogger(cfg *config.Config) (*zap.Logger, error) {
	zc := zap.NewProductionConfig()
	if cfg.AppEnv == "dev" {
		zc = zap.NewDevelopmentConfig()
	}
	if err := zc.Level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		return nil, fmt.Errorf("parse log level %q: %w", cfg.LogLevel, err)
	}
	log, err := zc.Build()
	if err != nil {
		return nil, fmt.Errorf("build logger: %w", err)
	}
	return log, nil
}

// unavailableAuth answers every call with err when Postgres did not open.
type unavailableAuth struct{ err error }

func (s unavailableAuth) RequestOTP(context.Context, dto.RequestOTPRequest) (*dto.RequestOTPResponse, error) {
	return nil, s.err
}
func (s unavailableAuth) VerifyOTP(context.Context, dto.VerifyOTPRequest, string, string) (*dto.AuthResponse, error) {
	return nil, s.err
}
func (s unavailableAuth) RegisterEmail(context.Context, dto.RegisterEmailRequest, string, string) (*dto.AuthResponse, error) {
	return nil, s.err
}
func (s unavailableAuth) LoginEmail(context.Context, dto.LoginEmailRequest, string, string) (*dto.AuthResponse, error) {
	return nil, s.err
}
func (s unavailableAuth) Refresh(context.Context, dto.RefreshRequest) (*dto.RefreshResponse, error) {
	return nil, s.err
}
func (s unavailableAuth) Logout(context.Context, string, string) error { return s.err }
func (s unavailableAuth) Me(context.Context, uuid.UUID) (*dto.UserResponse, error) {
	return nil, s.err
}

// unavailableTokens fails every token operation when the RSA keys did not load.
type unavailableTokens struct{ err error }

func (s unavailableTokens) GenerateAccessToken(uuid.UUID, string) (string, error) {
	return "", fmt.Errorf("access tokens unavailable: %w", s.err)
}
func (s unavailableTokens) GenerateRefreshToken() (string, string, error) {
	return "", "", fmt.Errorf("refresh tokens unavailable: %w", s.err)
}
func (s unavailableTokens) ValidateAccessToken(string) (*service.TokenClaims, error) {
	return nil, fmt.Errorf("access tokens unavailable: %w", s.err)
}
func (s unavailableTokens) HashToken(token string) string { return service.HashToken(token) }
func (s unavailableTokens) BlacklistToken(context.Context, string, time.Duration) error {
	return fmt.Errorf("access tokens unavailable: %w", s.err)
}
func (s unavailableTokens) IsBlacklisted(context.Context, string) (bool, error) {
	return false, fmt.Errorf("access tokens unavailable: %w", s.err)
}
