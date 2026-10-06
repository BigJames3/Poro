package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/shared-go/httpx"
	"github.com/poro/shared-go/jwtauth"

	"github.com/poro/video/internal/dto"
)

func TestVideosHandlerInitAndGet(t *testing.T) {
	vid := uuid.Must(uuid.NewV7())
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler(zap.NewNop())})
	app.Use(httpx.RequestID())
	app.Post("/uploads", func(c *fiber.Ctx) error {
		return httpx.WriteData(c, fiber.StatusCreated, dto.InitResponse{VideoID: vid.String(), UploadID: "u", PartSize: 8, Parts: []dto.PartURL{{PartNumber: 1, URL: "http://x"}}})
	})
	app.Get("/:id", func(c *fiber.Ctx) error {
		return httpx.WriteData(c, fiber.StatusOK, dto.VideoView{ID: c.Params("id"), Status: "processing"})
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/uploads", strings.NewReader(`{"title":"c","content_type":"video/mp4","size_bytes":10}`)), -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), vid.String())

	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/"+vid.String(), nil), -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestParseID(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler(zap.NewNop())})
	h := NewVideos(nil)
	h.principal = func(*fiber.Ctx) (*jwtauth.Claims, bool) { return nil, false }
	app.Get("/:id", h.Get)
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/not-a-uuid", nil), -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRequireUser(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler(zap.NewNop())})
	h := NewVideos(nil)
	h.principal = func(*fiber.Ctx) (*jwtauth.Claims, bool) { return nil, false }
	app.Post("/uploads", h.Init)
	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/uploads", strings.NewReader(`{}`)), -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
