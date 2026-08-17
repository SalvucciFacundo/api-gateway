// Package web embeds the built frontend assets so the gateway ships as a single
// self-contained binary (FE-002).
package web

import "embed"

// Dist holds the compiled frontend under dist/. The directory prefix ("dist/")
// is retained in the embedded paths, so callers strip it with fs.Sub before
// serving.
//
//go:embed dist
var Dist embed.FS
