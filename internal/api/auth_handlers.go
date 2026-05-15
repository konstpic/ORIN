package api

import (
	"encoding/json"
	"net/http"

	"github.com/k8s-ui/k8s-ui/internal/auth"
)

type loginRequest struct {
	Token string `json:"token"`
}

type loginResponse struct {
	Token string `json:"token"`
	Role  string `json:"role"`
}

// handleLogin validates the static admin token and sets it on a cookie.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if req.Token == "" || req.Token != s.opts.Config.AdminToken {
		writeError(w, http.StatusUnauthorized, "invalid_token", "")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "k8sui-token",
		Value:    req.Token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, loginResponse{Token: req.Token, Role: "admin"})
}

// handleUserInfo returns the authenticated principal.
func (s *Server) handleUserInfo(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"subject": u.Subject, "role": u.Role, "projects": u.Projects})
}
