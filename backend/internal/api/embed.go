package api

import (
	"embed"
	"io/fs"
)

// webdist holds the built frontend (web/dist), copied in by `make build`.
// When present, the SPA is served at / and the string-literal dashboard
// becomes the fallback. The .gitkeep keeps this embeddable in git checkouts
// where the frontend hasn't been built.
//
//go:embed all:webdist
var webDist embed.FS

// hasWebApp reports whether a built frontend is embedded.
func hasWebApp() bool {
	_, err := fs.Stat(webDist, "webdist/index.html")
	return err == nil
}

// webAppFS returns the embedded frontend rooted at its files.
func webAppFS() fs.FS {
	sub, err := fs.Sub(webDist, "webdist")
	if err != nil {
		return nil
	}
	return sub
}
