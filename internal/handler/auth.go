package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/SalvucciFacundo/api-gateway/internal/model"
	"github.com/SalvucciFacundo/api-gateway/internal/service"
)

// RegisterHandler handles POST /api/v1/auth/register (JWT-AUTH-001).
//
// It decodes the JSON body, delegates to the auth service, and maps the
// service's sentinel errors onto HTTP statuses: invalid email or weak password
// → 400, duplicate email → 409, anything unexpected → 500.
func RegisterHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req model.RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		user, err := svc.Register(r.Context(), req)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrInvalidEmail):
				writeError(w, http.StatusBadRequest, "invalid email")
			case errors.Is(err, service.ErrWeakPassword):
				writeError(w, http.StatusBadRequest, "weak password")
			case errors.Is(err, service.ErrEmailExists):
				writeError(w, http.StatusConflict, "email already exists")
			default:
				writeError(w, http.StatusInternalServerError, "internal error")
			}
			return
		}

		writeJSON(w, http.StatusCreated, model.UserResponse{ID: user.ID, Email: user.Email})
	}
}

// LoginHandler handles POST /api/v1/auth/login (JWT-AUTH-002). On success it
// returns the access/refresh pair; on invalid credentials it returns 401 with a
// generic message that never reveals which field was wrong.
func LoginHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req model.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		resp, err := svc.Login(r.Context(), req)
		if err != nil {
			if errors.Is(err, service.ErrInvalidCredentials) {
				writeError(w, http.StatusUnauthorized, "invalid credentials")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

// RefreshHandler handles POST /api/v1/auth/refresh (JWT-AUTH-003). A valid
// refresh token yields a fresh pair; an invalid or expired token yields 401.
func RefreshHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req model.RefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		resp, err := svc.Refresh(r.Context(), req)
		if err != nil {
			if errors.Is(err, service.ErrInvalidToken) {
				writeError(w, http.StatusUnauthorized, "invalid token")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, resp)
	}
}
