package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestHasCapability(t *testing.T) {
	cases := map[string]bool{
		"":                          false,
		"photo-posts":               true,
		"Photo-Posts":               true,
		" photo-posts ":             true,
		"stories, photo-posts":      true,
		"stories,PHOTO-POSTS,lives": true,
		"photo-posts-v2":            false,
		"photo":                     false,
		"photo posts":               false,
		"stories":                   false,
	}
	for header, want := range cases {
		if got := hasCapability(header, CapabilityPhotoPosts); got != want {
			t.Errorf("hasCapability(%q) = %v, want %v", header, got, want)
		}
	}
}

func TestPhotoPostsReadsTheRequestHeader(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error { return c.SendString(strconv.FormatBool(PhotoPosts(c))) })
	for header, want := range map[string]string{"": "false", "photo-posts": "true", "stories,Photo-Posts": "true"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if header != "" {
			req.Header.Set(CapabilitiesHeader, header)
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if string(body) != want {
			t.Errorf("header %q: PhotoPosts = %s, want %s", header, body, want)
		}
	}
}
