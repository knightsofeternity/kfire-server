// Package web embeds the built SvelteKit admin SPA so the server ships as a
// single binary. Run `pnpm build` in this directory before `go build`.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:build
var buildFS embed.FS

// Dist returns the built SPA as a filesystem rooted at the build output, or
// false when the SPA was not built (the embed contains only the placeholder).
func Dist() (fs.FS, bool) {
	sub, err := fs.Sub(buildFS, "build")
	if err != nil {
		return nil, false
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil, false
	}
	return sub, true
}
