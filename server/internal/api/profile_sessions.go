package api

import (
	"net/http"

	"github.com/pixelvide/aegis/server/internal/middleware"
	"github.com/pixelvide/aegis/server/internal/models"
)

// ─── Session Handlers ───────────────────────────────────────────────────────

// handleListSessions returns active sessions for the authenticated user.
func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	sessions, err := s.common.ListActiveSessions(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list sessions")
		return
	}
	if sessions == nil {
		sessions = []models.UserSession{}
	}

	// Mark current session
	currentJTI := middleware.JTIFromContext(r.Context())
	for i := range sessions {
		if sessions[i].JTI == currentJTI {
			sessions[i].Current = true
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"sessions": sessions,
	})
}

// handleRevokeSession revokes a specific session.
func (s *Server) handleRevokeSession(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	sessionID := r.PathValue("id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session ID is required")
		return
	}

	if err := s.common.RevokeSession(r.Context(), sessionID, user.ID); err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleRevokeAllSessions revokes all sessions except the current one.
func (s *Server) handleRevokeAllSessions(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	currentJTI := middleware.JTIFromContext(r.Context())
	if err := s.common.RevokeAllSessions(r.Context(), user.ID, currentJTI); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to revoke sessions")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "All other sessions have been revoked",
	})
}
