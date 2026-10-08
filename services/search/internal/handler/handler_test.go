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
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/shared-go/health"
	"github.com/poro/shared-go/httpx"
	"github.com/poro/shared-go/jwtauth"

	"github.com/poro/search/internal/handler"
	"github.com/poro/search/internal/index"
	"github.com/poro/search/internal/repository"
	"github.com/poro/search/internal/routes"
	"github.com/poro/search/internal/service"
	"github.com/poro/search/internal/testdb"
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
	svc := service.New(index.NewPostgres(testdb.Pool), func(k string) string { return "http://localhost:9000/poro-videos/" + k })
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler(zap.NewNop())})
	app.Use(httpx.RequestID())
	routes.Register(app, routes.Dependencies{
		Search:  handler.New(svc),
		Health:  health.New("search", "test", map[string]health.Check{"postgres": testdb.Pool.Ping}),
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
	for _, p := range []string{"/health", "/health/live", "/health/ready", "/api/v1/health"} {
		status, _, _ := a.get(p, nil)
		require.Equal(t, 200, status, p)
	}
	status, _, _ := a.get("/metrics", nil)
	require.Equal(t, 200, status)
}

func TestSearchWithAndWithoutAccount(t *testing.T) {
	a := newAPI(t)
	ctx := context.Background()
	word := "attieke" + uuid.NewString()[:6]
	author := uuid.Must(uuid.NewV7())
	name := "chef_" + uuid.NewString()[:6]
	require.NoError(t, repository.UpsertProfile(ctx, testdb.Pool, repository.Profile{UserID: author, Username: &name, UpdatedAt: time.Now()}))
	video := uuid.Must(uuid.NewV7())
	require.NoError(t, repository.UpsertReadyVideo(ctx, testdb.Pool, repository.Video{
		VideoID: video, AuthorID: author, Title: "Recette " + word, Hashtags: []string{word},
		ThumbnailKey: "videos/x/thumb.jpg", DurationMs: 1000, PublishedAt: time.Now(),
	}))

	user := uuid.Must(uuid.NewV7())
	for _, viewer := range []*uuid.UUID{nil, &user} {
		status, data, _ := a.get("/api/v1/search?q="+url.QueryEscape(word), viewer)
		require.Equal(t, 200, status)
		var page struct {
			Type  string `json:"type"`
			Items []struct {
				VideoID      string  `json:"video_id"`
				ThumbnailURL *string `json:"thumbnail_url"`
				Author       struct {
					Username string `json:"username"`
				} `json:"author"`
			} `json:"items"`
			NextCursor *string `json:"next_cursor"`
		}
		require.NoError(t, json.Unmarshal(data, &page))
		require.Equal(t, "videos", page.Type)
		require.Len(t, page.Items, 1)
		require.Equal(t, video.String(), page.Items[0].VideoID)
		require.Equal(t, "http://localhost:9000/poro-videos/videos/x/thumb.jpg", *page.Items[0].ThumbnailURL)
		require.Equal(t, name, page.Items[0].Author.Username)
		require.Nil(t, page.NextCursor)
	}

	status, data, _ := a.get("/api/v1/search?type=hashtags&q=%23"+word, nil)
	require.Equal(t, 200, status)
	require.Contains(t, string(data), word)
	status, data, _ = a.get("/api/v1/search?type=users&q=@"+name, nil)
	require.Equal(t, 200, status)
	require.Contains(t, string(data), name)
	status, data, _ = a.get("/api/v1/search/suggest?q="+name[:5], nil)
	require.Equal(t, 200, status)
	require.Contains(t, string(data), `"users":[`)

	for path, want := range map[string]string{
		"/api/v1/search":                     "invalid_query",
		"/api/v1/search?q=x&type=shops":      "invalid_type",
		"/api/v1/search?q=x&cursor=nope":     "invalid_cursor",
		"/api/v1/search/suggest?q=%20%20%20": "invalid_query",
	} {
		status, _, code := a.get(path, nil)
		require.Equal(t, 400, status, path)
		require.Equal(t, want, code, path)
	}
}
