// Package web holds the compiled single-page app, which Vite builds into
// web/dist. Until the UI is built, dist holds only .gitkeep and the server
// shows a placeholder page.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// FS returns the built UI files, rooted at dist/.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // "dist" is embedded above, so this cannot happen
	}
	return sub
}
