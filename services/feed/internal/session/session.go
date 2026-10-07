// Package session stores For You sessions and first-page caches in Redis.
package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Store keeps For You sessions: the ordered video ids served to one viewer.
// A session expires TTL after its last read.
type Store struct {
	rdb redis.UniversalClient
	ttl time.Duration
}

// NewStore builds a session store.
func NewStore(rdb redis.UniversalClient, ttl time.Duration) *Store {
	return &Store{rdb: rdb, ttl: ttl}
}

func sessionKey(viewer uuid.UUID, id string) string {
	return "feed:fy:" + viewer.String() + ":" + id
}

// Create saves ids under a new random session id.
func (s *Store) Create(ctx context.Context, viewer uuid.UUID, ids []uuid.UUID) (string, error) {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("session id: %w", err)
	}
	id := hex.EncodeToString(raw[:])
	parts := make([]string, len(ids))
	for i, v := range ids {
		parts[i] = v.String()
	}
	if err := s.rdb.Set(ctx, sessionKey(viewer, id), strings.Join(parts, ","), s.ttl).Err(); err != nil {
		return "", fmt.Errorf("save session: %w", err)
	}
	return id, nil
}

// Load returns the ids of a session and extends its life. ok is false when
// the session expired or belongs to another viewer.
func (s *Store) Load(ctx context.Context, viewer uuid.UUID, id string) ([]uuid.UUID, bool, error) {
	key := sessionKey(viewer, id)
	raw, err := s.rdb.GetEx(ctx, key, s.ttl).Result()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("load session: %w", err)
	}
	if raw == "" {
		return []uuid.UUID{}, true, nil
	}
	parts := strings.Split(raw, ",")
	ids := make([]uuid.UUID, 0, len(parts))
	for _, p := range parts {
		v, err := uuid.Parse(p)
		if err != nil {
			return nil, false, nil
		}
		ids = append(ids, v)
	}
	return ids, true, nil
}

// Cache holds short-lived serialized pages.
type Cache struct {
	rdb redis.UniversalClient
	ttl time.Duration
}

// NewCache builds a page cache.
func NewCache(rdb redis.UniversalClient, ttl time.Duration) *Cache {
	return &Cache{rdb: rdb, ttl: ttl}
}

// Get returns a cached value; ok is false on a miss.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	raw, err := c.rdb.Get(ctx, "feed:cache:"+key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("cache get: %w", err)
	}
	return raw, true, nil
}

// Set stores a value for the cache TTL.
func (c *Cache) Set(ctx context.Context, key string, value []byte) error {
	if err := c.rdb.Set(ctx, "feed:cache:"+key, value, c.ttl).Err(); err != nil {
		return fmt.Errorf("cache set: %w", err)
	}
	return nil
}
