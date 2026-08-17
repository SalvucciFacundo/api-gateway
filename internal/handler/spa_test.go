package handler

import (
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
)

// spaTestFS is a minimal embedded frontend build for the SPA handler tests.
var spaTestFS = fstest.MapFS{
	"index.html":     &fstest.MapFile{Data: []byte("<!doctype html><html><body>SPA</body></html>")},
	"assets/main.js": &fstest.MapFile{Data: []byte("console.log('hello');")},
}

// TestSPAHandlerServesRoot verifies FE-002/003: GET / serves index.html.
func TestSPAHandlerServesRoot(t *testing.T) {
	h := SPAHandler(spaTestFS)

	rec := doRequest(t, h, http.MethodGet, "/", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "SPA") {
		t.Errorf("body = %q, want to contain index.html content", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

// TestSPAHandlerServesStaticAsset verifies FE-004: static assets are served
// with the correct MIME type.
func TestSPAHandlerServesStaticAsset(t *testing.T) {
	h := SPAHandler(spaTestFS)

	rec := doRequest(t, h, http.MethodGet, "/assets/main.js", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "console.log") {
		t.Errorf("body = %q, want to contain the JS source", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Errorf("Content-Type = %q, want a javascript type", ct)
	}
}

// TestSPAHandlerFallback verifies FE-003: an unknown client route falls back to
// index.html so the SPA router can handle it.
func TestSPAHandlerFallback(t *testing.T) {
	h := SPAHandler(spaTestFS)

	rec := doRequest(t, h, http.MethodGet, "/dashboard", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "SPA") {
		t.Errorf("body = %q, want to contain index.html content (SPA fallback)", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}
