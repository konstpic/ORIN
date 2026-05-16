package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/k8s-ui/k8s-ui/internal/auth"
)

type loginRequest struct {
	Token string `json:"token"`
}

type loginResponse struct {
	Token       string `json:"token"`
	Role        string `json:"role"`
	DisplayName string `json:"displayName,omitempty"`
}

// handleLogin validates the token — first against the static admin token,
// then against user tokens stored in the database (bcrypt).
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if req.Token == "" {
		writeError(w, http.StatusUnauthorized, "invalid_token", "")
		return
	}

	// 1. Static admin token (bootstrap)
	if s.opts.Config.AdminToken != "" && req.Token == s.opts.Config.AdminToken {
		http.SetCookie(w, &http.Cookie{
			Name:     "k8sui-token",
			Value:    req.Token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		writeJSON(w, http.StatusOK, loginResponse{Token: req.Token, Role: "admin", DisplayName: "Administrator"})
		return
	}

	// 2. DB user token
	email, displayName, role, err := validateUserToken(r.Context(), s.opts.Store.Pool, req.Token)
	if err != nil {
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
	writeJSON(w, http.StatusOK, loginResponse{
		Token:       req.Token,
		Role:        role,
		DisplayName: displayName,
	})

	// Invalidate auth cache for this email
	_ = email
}

// validateUserToken checks the token against all user token_hashes in the DB.
func validateUserToken(ctx context.Context, pool *pgxpool.Pool, token string) (email, displayName, role string, err error) {
	// Fetch all users with token_hash
	rows, err := pool.Query(ctx, `
		SELECT email, display_name, role, token_hash
		FROM users WHERE token_hash IS NOT NULL AND active = true
	`)
	if err != nil {
		return "", "", "", err
	}
	defer rows.Close()

	for rows.Next() {
		var hash string
		if err := rows.Scan(&email, &displayName, &role, &hash); err != nil {
			continue
		}
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(token)) == nil {
			return email, displayName, role, nil
		}
	}
	return "", "", "", bcrypt.ErrMismatchedHashAndPassword
}

// handleUserInfo returns the authenticated principal.
func (s *Server) handleUserInfo(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	// Collect permission list for the frontend
	perms := make([]string, 0, len(u.Permissions))
	for p := range u.Permissions {
		perms = append(perms, string(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"subject":     u.Subject,
		"role":        u.Role,
		"projects":    u.Projects,
		"displayName": u.DisplayName,
		"permissions": perms,
	})
}
