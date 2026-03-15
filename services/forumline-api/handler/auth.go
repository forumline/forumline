package handler

import (
	"net/http"

	"github.com/forumline/forumline/services/forumline-api/store"
	shared "github.com/forumline/forumline/shared-go"
)

type AuthHandler struct {
	Store *store.Store
}

func NewAuthHandler(s *store.Store) *AuthHandler {
	return &AuthHandler{Store: s}
}

func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	clearPendingAuthCookie(w)
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (h *AuthHandler) HandleSession(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user": map[string]string{"id": userID},
	})
}

func clearPendingAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: "forumline_pending_auth", Value: "", Path: "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: true, MaxAge: -1,
	})
}
