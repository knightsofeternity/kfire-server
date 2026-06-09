package api

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"io"
	"net/http"

	_ "image/jpeg" // register the JPEG decoder for image.Decode/DecodeConfig

	"github.com/gofiber/fiber/v2"

	"github.com/knightsofeternity/kfire-server/internal/store"
)

// accentThemes are the selectable dominant colors. Must mirror the
// [data-accent='...'] themes defined in web/src/app.css.
var accentThemes = map[string]bool{
	"orange": true, "violet": true, "blue": true,
	"red": true, "green": true, "yellow": true,
}

const (
	maxLogoBytes = 2 << 20 // 2 MiB upload cap
	maxLogoDim   = 2048     // reject oversized images (decompression-bomb guard)
)

// GET /api/v1/admin/branding  (admin)
func (h *handlers) getBranding(c *fiber.Ctx) error {
	b, err := h.store.GetBranding(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"accent": b.Accent, "has_logo": b.HasLogo})
}

// PATCH /api/v1/admin/branding  (admin) - set the dominant accent theme.
func (h *handlers) setAccent(c *fiber.Ctx) error {
	var req struct {
		Accent string `json:"accent"`
	}
	if err := c.BodyParser(&req); err != nil {
		return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed", "invalid JSON body")
	}
	if !accentThemes[req.Accent] {
		return errorJSON(c, fiber.StatusUnprocessableEntity, "validation_failed", "unknown accent theme")
	}
	if err := h.store.SetAccent(c.Context(), req.Accent); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return errorJSON(c, fiber.StatusNotFound, "not_found", "no organization configured yet")
		}
		return err
	}
	return c.JSON(fiber.Map{"accent": req.Accent})
}

// POST /api/v1/admin/branding/logo  (admin, multipart form field "logo")
//
// Security posture for untrusted uploads:
//   - admin-only (guarded by the route middleware);
//   - hard size cap (declared size and actual bytes read);
//   - real type sniffed from magic bytes, PNG/JPEG only (SVG refused, since it
//     can carry script and would be an XSS vector when served inline);
//   - dimensions bounded via DecodeConfig before a full decode;
//   - the image is fully decoded then re-encoded to PNG, so the bytes we store
//     and later serve are produced by us, never the raw upload. This strips
//     EXIF, trailing data and polyglot payloads.
func (h *handlers) uploadLogo(c *fiber.Ctx) error {
	fh, err := c.FormFile("logo")
	if err != nil {
		return errorJSON(c, fiber.StatusBadRequest, "no_file", "expected a 'logo' file field")
	}
	if fh.Size > maxLogoBytes {
		return errorJSON(c, fiber.StatusRequestEntityTooLarge, "too_large", "logo must be 2 MB or less")
	}

	f, err := fh.Open()
	if err != nil {
		return err
	}
	defer f.Close()

	// Read at most the cap + 1 byte so an understated Content-Length can't slip
	// a larger body through.
	raw, err := io.ReadAll(io.LimitReader(f, maxLogoBytes+1))
	if err != nil {
		return err
	}
	if len(raw) > maxLogoBytes {
		return errorJSON(c, fiber.StatusRequestEntityTooLarge, "too_large", "logo must be 2 MB or less")
	}

	switch http.DetectContentType(raw) {
	case "image/png", "image/jpeg":
		// allowed
	default:
		return errorJSON(c, fiber.StatusUnsupportedMediaType, "bad_type", "logo must be a PNG or JPEG image")
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width == 0 || cfg.Height == 0 ||
		cfg.Width > maxLogoDim || cfg.Height > maxLogoDim {
		return errorJSON(c, fiber.StatusUnprocessableEntity, "bad_image", "logo image is invalid or too large")
	}

	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return errorJSON(c, fiber.StatusUnprocessableEntity, "bad_image", "could not decode the image")
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}

	if err := h.store.SetOrgLogo(c.Context(), "image/png", buf.Bytes()); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return errorJSON(c, fiber.StatusNotFound, "not_found", "no organization configured yet")
		}
		return err
	}
	return c.JSON(fiber.Map{"ok": true})
}

// DELETE /api/v1/admin/branding/logo  (admin)
func (h *handlers) deleteLogo(c *fiber.Ctx) error {
	if err := h.store.DeleteOrgLogo(c.Context()); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// GET /img/org/logo  (public) - serves the uploaded org logo, if any.
func (h *handlers) orgLogo(c *fiber.Ctx) error {
	img, err := h.store.GetOrgLogo(c.Context())
	if errors.Is(err, store.ErrNotFound) {
		return c.SendStatus(fiber.StatusNotFound)
	}
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, img.ContentType)
	c.Set("X-Content-Type-Options", "nosniff")
	// Short cache: the admin can replace the logo and expect it to update soon.
	c.Set(fiber.HeaderCacheControl, "public, max-age=300")
	return c.Send(img.Data)
}
