package api

import (
	"database/sql"
	"net/http"
	"time"
)

// HealthResponse is the JSON body returned by health check endpoints.
type HealthResponse struct {
	Status    string `json:"status"`
	DB        string `json:"db"`
	Timestamp string `json:"timestamp"`
}

// HealthHandler holds the database pool for health checks.
type HealthHandler struct {
	db *sql.DB
}

// NewHealthHandler creates a new health check handler.
func NewHealthHandler(db *sql.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

// HandleHealthz is a liveness probe. Returns 200 if the server is running
// and the database is reachable, 503 otherwise.
func (h *HealthHandler) HandleHealthz(w http.ResponseWriter, r *http.Request) {
	resp := HealthResponse{
		Status:    "ok",
		DB:        "ok",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	if err := h.db.PingContext(r.Context()); err != nil {
		resp.Status = "degraded"
		resp.DB = "unreachable"
		writeJSON(w, http.StatusServiceUnavailable, resp)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleReadyz is a readiness probe. Same logic as healthz — ready when
// the database is reachable.
func (h *HealthHandler) HandleReadyz(w http.ResponseWriter, r *http.Request) {
	h.HandleHealthz(w, r)
}
