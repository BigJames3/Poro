package handler_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/shared-go/health"
	"github.com/poro/shared-go/httpx"
	"github.com/poro/shared-go/jwtauth"

	"github.com/poro/feed/internal/dto"
	"github.com/poro/feed/internal/handler"
	"github.com/poro/feed/internal/model"
	"github.com/poro/feed/internal/repository"
	"github.com/poro/feed/internal/routes"
	"github.com/poro/feed/internal/service"
	"github.com/poro/feed/internal/session"
	"github.com/poro/feed/internal/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }

type api struct {
	t   *testing.T
	app *fiber.App
	key *rsa.PrivateKey
}

func newAPI(t *testing.T) *api {
	t.Helper()
	testdb.Available(t)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": "test", "use": "sig", "alg": "RS256",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	t.Cleanup(jwks.Close)
	verifier, err := jwtauth.NewVerifier(jwtauth.Config{JWKSURL: jwks.URL})
	require.NoError(t, err)

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	svc := service.New(testdb.Pool, session.NewStore(rdb, model.SessionTTL), session.NewCache(rdb, model.FirstPageTTL),
		func(k string) string { return "http://localhost:9000/poro-videos/" + k }, zap.NewNop())

	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler(zap.NewNop())})
	app.Use(httpx.RequestID())
	routes.Register(app, routes.Dependencies{
		Feed:    handler.New(svc),
		Health:  health.New("feed", "test", map[string]health.Check{"postgres": testdb.Pool.Ping}),
		Metrics: func(c *fiber.Ctx) error { return c.SendString("metrics") },
		JWT:     verifier,
	})
	return &api{t: t, app: app, key: key}
}

func (a *api) get(path string, user *uuid.UUID) (int, json.RawMessage, string) {
	a.t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	if user != nil {
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"sub": user.String(), "sid": uuid.NewString(), "jti": uuid.NewString(),
			"iss": jwtauth.Issuer, "aud": jwtauth.Audience, "exp": time.Now().Add(time.Hour).Unix(),
		})
		tok.Header["kid"] = "test"
		signed, err := tok.SignedString(a.key)
		require.NoError(a.t, err)
		req.Header.Set("Authorization", "Bearer "+signed)
	}
	res, err := a.app.Test(req, 10000)
	require.NoError(a.t, err)
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	require.NoError(a.t, err)
	var env struct {
		Data  json.RawMessage `json:"data"`
		Error *struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if len(raw) > 0 && raw[0] == '{' {
		require.NoError(a.t, json.Unmarshal(raw, &env))
	}
	code := ""
	if env.Error != nil {
		code = env.Error.Code
	}
	return res.StatusCode, env.Data, code
}

func TestProbes(t *testing.T) {
	a := newAPI(t)
	for _, p := range []string{"/health", "/health/live", "/health/ready", "/api/v1/health", "/api/v1/health/ready"} {
		status, _, _ := a.get(p, nil)
		require.Equal(t, 200, status, p)
	}
	status, _, _ := a.get("/metrics", nil)
	require.Equal(t, 200, status)
}

func TestFeedsOverHTTP(t *testing.T) {
	a := newAPI(t)
	ctx := context.Background()
	viewer, author := uuid.New(), uuid.New()
	vid := uuid.Must(uuid.NewV7())
	require.NoError(t, repository.UpsertReadyVideo(ctx, testdb.Pool, repository.Video{
		VideoID: vid, AuthorID: author, Title: "Danse", Hashtags: []string{"abidjan"},
		ThumbnailKey: "videos/a/v/thumb.jpg", HLSKey: "videos/a/v/hls/master.m3u8", DurationMs: 15000,
		PublishedAt: time.Now().Add(-time.Hour),
	}))
	require.NoError(t, repository.AddFollow(ctx, testdb.Pool, viewer, author, time.Now()))
	require.NoError(t, repository.AddStats(ctx, testdb.Pool, vid, 3, 1, 0))
	name := "awa.k"
	require.NoError(t, repository.UpsertAuthor(ctx, testdb.Pool, author, &name, nil, nil, time.Now()))

	for _, path := range []string{"/api/v1/feed/following", "/api/v1/feed/trending", "/api/v1/feed/for-you"} {
		status, _, code := a.get(path, nil)
		require.Equal(t, 401, status, path)
		require.Equal(t, "unauthorized", code)

		status, data, _ := a.get(path+"?limit=5", &viewer)
		require.Equal(t, 200, status, path)
		var page dto.Page
		require.NoError(t, json.Unmarshal(data, &page))
		require.NotEmpty(t, page.Items, path)
		it := page.Items[0]
		require.Equal(t, vid.String(), it.VideoID)
		require.Equal(t, "http://localhost:9000/poro-videos/videos/a/v/thumb.jpg", it.ThumbnailURL)
		require.Equal(t, int64(3), it.Stats.Likes)
		require.Equal(t, "awa.k", *it.Author.Username)
		require.Nil(t, it.Author.AvatarURL)
	}

	status, _, code := a.get("/api/v1/feed/following?cursor=bad", &viewer)
	require.Equal(t, 400, status)
	require.Equal(t, "invalid_cursor", code)
}
