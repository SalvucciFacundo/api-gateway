package handler

import (
	"io/fs"
	"net/http"
	"strings"
)

// SPAHandler serves an embedded frontend build rooted at fsys.
//
// Static files are served as-is with their detected MIME types (FE-004). Any
// path that does not resolve to a file falls back to index.html so client-side
// routing keeps working on refresh/deep links (FE-003). Path traversal is
// prevented by http.FileServer/http.FS, which clean the request path before
// opening.
func SPAHandler(fsys fs.FS) http.HandlerFunc {
	fileServer := http.FileServer(http.FS(fsys))

	return func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name != "" {
			if _, err := fs.Stat(fsys, name); err != nil {
				// Unknown client route: let the SPA router resolve it.
				r.URL.Path = "/"
			}
		}
		fileServer.ServeHTTP(w, r)
	}
}
