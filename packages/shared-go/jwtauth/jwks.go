package jwtauth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"
)

const (
	minRSABits    = 2048
	maxJWKSBytes  = 64 << 10
	defaultMaxAge = 5 * time.Minute
	defaultMinGap = 30 * time.Second
)

// ErrKeysUnavailable is returned when no signing key can be loaded from the JWKS endpoint.
var ErrKeysUnavailable = errors.New("jwks unavailable")

type jwk struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// keySet caches the RSA keys of the auth service. It refreshes after maxAge,
// and on an unknown kid at most once per minGap, so forged kids cannot flood auth.
type keySet struct {
	url     string
	client  *http.Client
	maxAge  time.Duration
	minGap  time.Duration
	now     func() time.Time
	mu      sync.Mutex
	keys    map[string]*rsa.PublicKey
	fetched time.Time
	tried   time.Time
}

func (s *keySet) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	stale := s.keys == nil || now.Sub(s.fetched) >= s.maxAge
	_, known := s.keys[kid]
	if (stale || !known) && now.Sub(s.tried) >= s.minGap {
		s.tried = now
		keys, err := s.fetch(ctx)
		if err == nil {
			s.keys, s.fetched = keys, now
		} else if s.keys == nil {
			return nil, fmt.Errorf("%w: %w", ErrKeysUnavailable, err)
		}
	}
	if s.keys == nil {
		return nil, ErrKeysUnavailable
	}
	key, ok := s.keys[kid]
	if !ok {
		return nil, fmt.Errorf("unknown key id %q", kid)
	}
	return key, nil
}

func (s *keySet) fetch(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return nil, fmt.Errorf("build jwks request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	res, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch jwks: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch jwks: http %d", res.StatusCode)
	}
	var doc struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, maxJWKSBytes)).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode jwks: %w", err)
	}
	keys := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		if k.Kty != "RSA" || k.Kid == "" || (k.Use != "" && k.Use != "sig") || (k.Alg != "" && k.Alg != "RS256") {
			continue
		}
		pub, err := k.rsaKey()
		if err != nil {
			return nil, fmt.Errorf("jwk %q: %w", k.Kid, err)
		}
		keys[k.Kid] = pub
	}
	if len(keys) == 0 {
		return nil, errors.New("jwks has no RS256 signing key")
	}
	return keys, nil
}

func (k jwk) rsaKey() (*rsa.PublicKey, error) {
	nb, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("decode modulus: %w", err)
	}
	eb, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("decode exponent: %w", err)
	}
	if len(eb) == 0 || len(eb) > 4 {
		return nil, errors.New("invalid exponent")
	}
	n := new(big.Int).SetBytes(nb)
	if n.BitLen() < minRSABits {
		return nil, fmt.Errorf("key is %d bits, need at least %d", n.BitLen(), minRSABits)
	}
	e := int(new(big.Int).SetBytes(eb).Int64())
	if e < 3 || e%2 == 0 {
		return nil, errors.New("invalid exponent")
	}
	return &rsa.PublicKey{N: n, E: e}, nil
}
