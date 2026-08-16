package model

import (
	"encoding/json"
	"reflect"
	"testing"
)

// assertJSONRoundTrip marshals in, checks the snake_case JSON keys carry the
// expected values, then unmarshals back and verifies equality.
func assertJSONRoundTrip[T any](t *testing.T, in T, keys map[string]string) {
	t.Helper()

	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal(%T) error = %v", in, err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal into map error = %v", err)
	}
	for k, want := range keys {
		got, ok := raw[k]
		if !ok {
			t.Errorf("json missing key %q: %s", k, data)
			continue
		}
		if got != want {
			t.Errorf("json[%q] = %v, want %v", k, got, want)
		}
	}

	var out T
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("round-trip Unmarshal(%T) error = %v", in, err)
	}
	if !reflect.DeepEqual(out, in) {
		t.Errorf("round-trip(%T) = %+v, want %+v", in, out, in)
	}
}

func TestAuthModelsJSON(t *testing.T) {
	assertJSONRoundTrip(t, RegisterRequest{Email: "user@example.com", Password: "securePass123"}, map[string]string{
		"email":    "user@example.com",
		"password": "securePass123",
	})
	assertJSONRoundTrip(t, LoginRequest{Email: "user@example.com", Password: "securePass123"}, map[string]string{
		"email":    "user@example.com",
		"password": "securePass123",
	})
	assertJSONRoundTrip(t, AuthResponse{AccessToken: "access-token", RefreshToken: "refresh-token"}, map[string]string{
		"access_token":  "access-token",
		"refresh_token": "refresh-token",
	})
	assertJSONRoundTrip(t, RefreshRequest{RefreshToken: "refresh-token"}, map[string]string{
		"refresh_token": "refresh-token",
	})
	assertJSONRoundTrip(t, UserResponse{ID: "uuid-123", Email: "user@example.com"}, map[string]string{
		"id":    "uuid-123",
		"email": "user@example.com",
	})
}
