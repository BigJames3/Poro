// Package revocation is the registry of revoked sessions that every Poro
// service shares (ADR-0016). The auth service writes the session id (the sid
// claim) of each session it revokes; every service refuses the access tokens
// of a session found in the registry. An entry lives as long as an access
// token, so the registry never holds more than the sessions revoked during the
// last token lifetime.
package revocation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// KeyPrefix prefixes the key of each revoked session. Every implementation of
// the registry, in Go, TypeScript or Python, uses it.
const KeyPrefix = "poro:revoked:sid:"

// ErrUnavailable wraps every registry failure. Callers refuse the request
// rather than accept a token they cannot check.
var ErrUnavailable = errors.New("revocation registry unavailable")

// Store reads and writes the registry in Redis.
type Store struct {
	client *redis.Client
}

// New wraps a Redis client.
func New(client *redis.Client) *Store {
	return &Store{client: client}
}

// Open connects to the registry at a redis:// or rediss:// URL. Every
// authenticated request reads the registry, so an outage must fail fast:
// unless the URL sets them, timeouts are short and a failed call is retried
// once.
func Open(url string) (*Store, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("revocation registry url: %w", err)
	}
	if opts.DialTimeout == 0 {
		opts.DialTimeout = time.Second
	}
	if opts.ReadTimeout == 0 {
		opts.ReadTimeout = 500 * time.Millisecond
	}
	if opts.WriteTimeout == 0 {
		opts.WriteTimeout = 500 * time.Millisecond
	}
	if opts.MaxRetries == 0 {
		opts.MaxRetries = 1
	}
	return New(redis.NewClient(opts)), nil
}

// Key is the registry key of a session.
func Key(sessionID uuid.UUID) string {
	return KeyPrefix + sessionID.String()
}

// Revoke records sessions as revoked for ttl, which must cover the lifetime
// of the access tokens already issued for them.
func (s *Store) Revoke(ctx context.Context, ttl time.Duration, sessionIDs ...uuid.UUID) error {
	if ttl <= 0 {
		return fmt.Errorf("revoke sessions: ttl must be positive, got %s", ttl)
	}
	if len(sessionIDs) == 0 {
		return nil
	}
	pipe := s.client.Pipeline()
	for _, id := range sessionIDs {
		pipe.Set(ctx, Key(id), "1", ttl)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("%w: revoke sessions: %w", ErrUnavailable, err)
	}
	return nil
}

// IsRevoked reports whether a session is in the registry.
func (s *Store) IsRevoked(ctx context.Context, sessionID uuid.UUID) (bool, error) {
	n, err := s.client.Exists(ctx, Key(sessionID)).Result()
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return n > 0, nil
}

// Ping checks that the registry answers.
func (s *Store) Ping(ctx context.Context) error {
	if err := s.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return nil
}

// Close releases the connection pool.
func (s *Store) Close() error {
	return s.client.Close()
}

// CheckURL validates the registry URL of a service that reads the registry.
// Outside dev it is required; in dev it may be empty, which disables the
// check for a service started on its own.
func CheckURL(url string, dev bool) error {
	if url == "" {
		if dev {
			return nil
		}
		return errors.New("REVOCATION_REDIS_URL is required outside dev: revoked sessions must be refused")
	}
	if _, err := redis.ParseURL(url); err != nil {
		return fmt.Errorf("REVOCATION_REDIS_URL must be a redis:// or rediss:// URL: %w", err)
	}
	return nil
}
