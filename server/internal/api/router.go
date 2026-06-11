// Package api provides the HTTP handlers for the Aegis server.
package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	authpkg "github.com/pixelvide/aegis/server/internal/auth"
	"github.com/pixelvide/aegis/server/internal/config"
	"github.com/pixelvide/aegis/server/internal/middleware"
	"github.com/pixelvide/aegis/server/internal/store"
)

// Server holds the HTTP handler dependencies.
type Server struct {
	common *store.CommonStore
	auth   *authpkg.Service
	config *config.Config
	mux    *http.ServeMux
}

// New creates a new API server.
func New(common *store.CommonStore, authSvc *authpkg.Service, cfg *config.Config) *Server {
	srv := &Server{
		common: common,
		auth:   authSvc,
		config: cfg,
		mux:    http.NewServeMux(),
	}
	srv.routes()
	return srv
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Security headers
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

	// CORS
	origin := r.Header.Get("Origin")
	if origin != "" && s.isAllowedOrigin(origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Org-ID, X-Org-Slug")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "86400")
	}

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	s.mux.ServeHTTP(w, r)
}

// authMiddleware wraps a handler with authentication.
func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	authMw := middleware.Auth(s.auth, s.common)
	return func(w http.ResponseWriter, r *http.Request) {
		authMw(http.HandlerFunc(next)).ServeHTTP(w, r)
	}
}

// protectedMiddleware wraps a handler with auth + tenant resolution.
func (s *Server) protectedMiddleware(next http.HandlerFunc) http.HandlerFunc {
	authMw := middleware.Auth(s.auth, s.common)
	tenantMw := middleware.TenantResolver(s.common)
	return func(w http.ResponseWriter, r *http.Request) {
		authMw(tenantMw(http.HandlerFunc(next))).ServeHTTP(w, r)
	}
}

// agentMiddleware wraps a handler with Bearer token auth (not JWT cookies).
func (s *Server) agentMiddleware(next http.HandlerFunc) http.HandlerFunc {
	tokenMw := middleware.TokenAuth(s.common, s.config)
	return func(w http.ResponseWriter, r *http.Request) {
		tokenMw(http.HandlerFunc(next)).ServeHTTP(w, r)
	}
}

// routes registers all API endpoints.
func (s *Server) routes() {
	// ─── Auth (public) ──────────────────────────────────────────────
	s.mux.HandleFunc("POST /api/v1/auth/register", s.handleRegister)
	s.mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/v1/auth/logout", s.handleLogout)
	s.mux.HandleFunc("GET /api/v1/auth/me", s.authMiddleware(s.handleMe))

	// ─── Orgs (authenticated, no tenant context) ────────────────────
	s.mux.HandleFunc("POST /api/v1/orgs", s.authMiddleware(s.handleCreateOrg))
	s.mux.HandleFunc("GET /api/v1/orgs", s.authMiddleware(s.handleListOrgs))
	s.mux.HandleFunc("GET /api/v1/orgs/{slug}", s.authMiddleware(s.handleGetOrg))

	// ─── Feature flags (authenticated) ──────────────────────────────
	s.mux.HandleFunc("GET /api/v1/config/features", s.authMiddleware(s.handleListFeatureFlags))

	// ─── Tenant-scoped routes (auth + org context) ──────────────────
	// Scans (read-only, legacy data)
	s.mux.HandleFunc("GET /api/v1/scans", s.protectedMiddleware(s.handleListScans))
	s.mux.HandleFunc("GET /api/v1/scans/{id}", s.protectedMiddleware(s.handleGetScan))

	// Findings
	s.mux.HandleFunc("GET /api/v1/findings", s.protectedMiddleware(s.handleListFindings))
	s.mux.HandleFunc("GET /api/v1/findings/{id}", s.protectedMiddleware(s.handleGetFinding))
	s.mux.HandleFunc("PATCH /api/v1/findings/{id}", s.protectedMiddleware(s.handleUpdateFinding))
	s.mux.HandleFunc("GET /api/v1/findings/{id}/exploits", s.protectedMiddleware(s.handleListExploits))
	s.mux.HandleFunc("GET /api/v1/findings/{id}/exploits/{eid}", s.protectedMiddleware(s.handleGetExploit))

	// Dashboard
	s.mux.HandleFunc("GET /api/v1/dashboard/stats", s.protectedMiddleware(s.handleDashboardStats))

	// Projects
	s.mux.HandleFunc("POST /api/v1/projects", s.protectedMiddleware(s.handleCreateProject))
	s.mux.HandleFunc("GET /api/v1/projects", s.protectedMiddleware(s.handleListProjects))
	s.mux.HandleFunc("GET /api/v1/projects/{slug}", s.protectedMiddleware(s.handleGetProject))

	// Members
	s.mux.HandleFunc("GET /api/v1/members", s.protectedMiddleware(s.handleListMembers))
	s.mux.HandleFunc("POST /api/v1/members/invite", s.protectedMiddleware(s.handleInviteMember))
	s.mux.HandleFunc("DELETE /api/v1/members/{userId}", s.protectedMiddleware(s.handleRemoveMember))

	// ─── Agent Ingest API (Bearer token auth) ───────────────────────
	s.mux.HandleFunc("POST /api/v1/agent/findings", s.agentMiddleware(s.handleAgentCreateFinding))
	s.mux.HandleFunc("GET /api/v1/agent/findings", s.agentMiddleware(s.handleAgentListFindings))
	s.mux.HandleFunc("PATCH /api/v1/agent/findings/{id}", s.agentMiddleware(s.handleAgentUpdateFinding))
	s.mux.HandleFunc("POST /api/v1/agent/findings/{id}/exploits", s.agentMiddleware(s.handleAgentCreateExploit))

	// ─── Token Management (user auth + org context) ─────────────────
	s.mux.HandleFunc("POST /api/v1/tokens", s.protectedMiddleware(s.handleCreateToken))
	s.mux.HandleFunc("GET /api/v1/tokens", s.protectedMiddleware(s.handleListTokens))
	s.mux.HandleFunc("DELETE /api/v1/tokens/{id}", s.protectedMiddleware(s.handleRevokeToken))

	// ─── Swagger / OpenAPI (public) ──────────────────────────────────
	s.mux.HandleFunc("GET /api/v1/docs", s.handleSwaggerUI)
	s.mux.HandleFunc("GET /api/v1/docs/openapi.yaml", s.handleSwaggerSpec)
}

// handleListFeatureFlags returns all feature flags.
func (s *Server) handleListFeatureFlags(w http.ResponseWriter, r *http.Request) {
	flags, err := s.common.ListFeatureFlags(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list feature flags")
		return
	}
	if flags == nil {
		flags = []store.FeatureFlag{}
	}
	writeJSON(w, http.StatusOK, flags)
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func (s *Server) isAllowedOrigin(origin string) bool {
	for _, allowed := range s.config.AllowedOrigins {
		if allowed == origin {
			return true
		}
	}
	return false
}

// tenantStore extracts the tenant-scoped store from the request context.
// Must only be called from handlers wrapped with protectedMiddleware.
func tenantStore(r *http.Request) store.Store {
	return middleware.TenantStoreFromContext(r.Context())
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("json encode error: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	// Limit request body to 1MB to prevent DoS
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// pathParam extracts a URL path parameter. Go 1.22+ net/http uses {name} syntax.
func pathParam(r *http.Request, name string) string {
	return r.PathValue(name)
}

// queryParam returns a query parameter, trimmed.
func queryParam(r *http.Request, name string) string {
	return strings.TrimSpace(r.URL.Query().Get(name))
}
