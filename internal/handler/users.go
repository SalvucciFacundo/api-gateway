package handler

import (
	"errors"
	"net/http"

	"github.com/SalvucciFacundo/api-gateway/internal/middleware"
	"github.com/SalvucciFacundo/api-gateway/internal/model"
	"github.com/SalvucciFacundo/api-gateway/internal/service"
	"github.com/SalvucciFacundo/api-gateway/internal/store"
)

// GetCurrentUserHandler handles GET /api/v1/users/me (JWT-AUTH-005). It reads
// the authenticated user ID from the request context (set by the Auth
// middleware) and returns that user's public data.
func GetCurrentUserHandler(svc *service.AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		user, err := svc.GetUser(r.Context(), userID)
		if err != nil {
			if errors.Is(err, store.ErrUserNotFound) {
				writeError(w, http.StatusNotFound, "user not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, model.UserResponse{ID: user.ID, Email: user.Email})
	}
}
