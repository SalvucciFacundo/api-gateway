package handler

import (
	"encoding/json"
	"net/http"
)

// writeJSON serializes v as JSON with the given status code and a JSON content
// type. Callers are expected to have validated v before calling.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError emits a JSON error body of the form {"error": message} with the
// given status code, matching the gateway-wide error envelope used by the
// middleware.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
