// Package static carries the built admin UI inside the binary. `npm run
// build` in web/ writes into dist/; the Go build embeds whatever is there.
package static

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// FS returns the built UI rooted at dist/.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
