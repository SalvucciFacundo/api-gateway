package web

import (
	"io/fs"
	"testing"
)

// TestDistEmbedsIndexHTML verifies FE-002: the embedded filesystem contains the
// built frontend's index.html at dist/index.html with non-empty content.
func TestDistEmbedsIndexHTML(t *testing.T) {
	data, err := fs.ReadFile(Dist, "dist/index.html")
	if err != nil {
		t.Fatalf("read dist/index.html: %v", err)
	}
	if len(data) == 0 {
		t.Error("dist/index.html is empty, want non-empty content")
	}
}
