package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/store"
)

// imageSizes is the Discord CDN ?size= we cache per kind (icons small, covers large).
var imageSizes = map[string]string{"icon": "128", "cover": "512"}

// maxImageBytes bounds a single cached image (defensive).
const maxImageBytes = 2 << 20 // 2 MiB

var imageHTTP = &http.Client{Timeout: 15 * time.Second}

// GET /img/games/:id/:kind  (public) — lazy image cache.
//
// Serves a game's icon or cover. On the first request the image is fetched
// from Discord's CDN and stored in Postgres; subsequent requests serve the
// cached copy, so storage scales with the games actually displayed.
func (h *handlers) gameImage(c *fiber.Ctx) error {
	id := c.Params("id")
	kind := c.Params("kind")
	size, ok := imageSizes[kind]
	if !ok {
		return c.SendStatus(fiber.StatusNotFound)
	}
	key := id + ":" + kind

	serve := func(img store.CachedImage) error {
		c.Set(fiber.HeaderContentType, img.ContentType)
		c.Set(fiber.HeaderCacheControl, "public, max-age=31536000, immutable")
		return c.Send(img.Data)
	}

	// Cache hit.
	if img, err := h.store.GetCachedImage(c.Context(), key); err == nil {
		return serve(img)
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}

	// Miss: resolve the source Discord URL.
	game, err := h.store.GetGameByID(c.Context(), id)
	if err != nil {
		return c.SendStatus(fiber.StatusNotFound)
	}
	var src *string
	if kind == "cover" {
		src = game.CoverURL
	} else {
		src = game.IconURL
	}
	if src == nil {
		return c.SendStatus(fiber.StatusNotFound)
	}

	img, err := fetchImage(c.UserContext(), *src+"?size="+size)
	if err != nil {
		// Fall back to redirecting to the source so the image still shows.
		return c.Redirect(*src, fiber.StatusFound)
	}
	// Best-effort cache write; serving doesn't depend on it.
	_ = h.store.PutCachedImage(c.Context(), key, img.ContentType, img.Data)
	return serve(img)
}

func fetchImage(ctx context.Context, url string) (store.CachedImage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return store.CachedImage{}, err
	}
	req.Header.Set("User-Agent", "kfire-server")
	resp, err := imageHTTP.Do(req)
	if err != nil {
		return store.CachedImage{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return store.CachedImage{}, errors.New("upstream image not ok")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes))
	if err != nil {
		return store.CachedImage{}, err
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "image/png"
	}
	return store.CachedImage{ContentType: ct, Data: data}, nil
}
