// Package api provides the HTTP handlers for the Aegis server.
package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	authpkg "github.com/pixelvide/aegis/server/internal/auth"
	"github.com/pixelvide/aegis/server/internal/cache"
	"github.com/pixelvide/aegis/server/internal/config"
	"github.com/pixelvide/aegis/server/internal/email"
	"github.com/pixelvide/aegis/server/internal/middleware"
	"github.com/pixelvide/aegis/server/internal/store"
)

// Server holds the HTTP handler dependencies.
type Server struct {
	common *store.CommonStore
	auth   *authpkg.Service
	email  *email.Service
	cache  *cache.Client
	config *config.Config
	mux    *http.ServeMux
}

// New creates a new API server.
func New(common *store.CommonStore, authSvc *authpkg.Service, emailSvc *email.Service, cacheClient *cache.Client, cfg *config.Config) *Server {
	srv := &Server{
		common: common,
		auth:   authSvc,
		email:  emailSvc,
		cache:  cacheClient,
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
	authMw := middleware.Auth(s.auth, s.common, s.cache)
	return func(w http.ResponseWriter, r *http.Request) {
		authMw(http.HandlerFunc(next)).ServeHTTP(w, r)
	}
}

// verifiedMiddleware wraps a handler with auth + primary email verification.
// Returns 403 if the user's primary email is not verified.
func (s *Server) verifiedMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		user := middleware.UserFromContext(r.Context())
		if user != nil && !user.EmailVerified {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":"email_not_verified","message":"Please verify your email address before continuing"}`))
			return
		}
		next(w, r)
	})
}

// protectedMiddleware wraps a handler with auth + email verification + tenant resolution.
func (s *Server) protectedMiddleware(next http.HandlerFunc) http.HandlerFunc {
	authMw := middleware.Auth(s.auth, s.common, s.cache)
	tenantMw := middleware.TenantResolver(s.common)
	return func(w http.ResponseWriter, r *http.Request) {
		authMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := middleware.UserFromContext(r.Context())
			if user != nil && !user.EmailVerified {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"error":"email_not_verified","message":"Please verify your email address before continuing"}`))
				return
			}
			tenantMw(http.HandlerFunc(next)).ServeHTTP(w, r)
		})).ServeHTTP(w, r)
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

	// Password reset (public)
	s.mux.HandleFunc("POST /api/v1/auth/forgot-password", s.handleForgotPassword)
	s.mux.HandleFunc("POST /api/v1/auth/reset-password", s.handleResetPassword)

	// Password change (requires verified email)
	s.mux.HandleFunc("POST /api/v1/auth/change-password", s.verifiedMiddleware(s.handleChangePassword))

	// MFA validation during login (public — uses MFA token, not full auth)
	s.mux.HandleFunc("POST /api/v1/auth/mfa/validate", s.handleMFAValidate)
	s.mux.HandleFunc("POST /api/v1/auth/mfa/send-email-otp", s.handleSendEmailOTP)

	// Email verification (public — uses token from email link)
	s.mux.HandleFunc("POST /api/v1/auth/verify-email", s.handleVerifyEmail)

	// ─── Profile (authenticated, no tenant context) ──────────────────
	// Emails — accessible WITHOUT verified email (needed to verify!)
	s.mux.HandleFunc("GET /api/v1/profile/emails", s.authMiddleware(s.handleListEmails))
	s.mux.HandleFunc("POST /api/v1/profile/emails", s.authMiddleware(s.handleAddEmail))
	s.mux.HandleFunc("DELETE /api/v1/profile/emails/{id}", s.authMiddleware(s.handleRemoveEmail))
	s.mux.HandleFunc("POST /api/v1/profile/emails/{id}/set-primary", s.authMiddleware(s.handleSetPrimaryEmail))
	s.mux.HandleFunc("POST /api/v1/profile/emails/{id}/send-verification", s.authMiddleware(s.handleSendEmailVerification))

	// MFA devices (requires verified email)
	s.mux.HandleFunc("GET /api/v1/profile/mfa/devices", s.verifiedMiddleware(s.handleListMFADevices))
	s.mux.HandleFunc("POST /api/v1/profile/mfa/devices/totp", s.verifiedMiddleware(s.handleAddTOTPDevice))
	s.mux.HandleFunc("POST /api/v1/profile/mfa/devices/totp/{id}/verify", s.verifiedMiddleware(s.handleVerifyTOTPDevice))
	s.mux.HandleFunc("POST /api/v1/profile/mfa/devices/email", s.verifiedMiddleware(s.handleAddEmailMFADevice))
	s.mux.HandleFunc("POST /api/v1/profile/mfa/devices/email/{id}/verify", s.verifiedMiddleware(s.handleVerifyEmailMFADevice))
	s.mux.HandleFunc("DELETE /api/v1/profile/mfa/devices/{id}", s.verifiedMiddleware(s.handleRemoveMFADevice))

	// MFA toggle (requires verified email)
	s.mux.HandleFunc("POST /api/v1/profile/mfa/enable", s.verifiedMiddleware(s.handleEnableMFA))
	s.mux.HandleFunc("POST /api/v1/profile/mfa/disable", s.verifiedMiddleware(s.handleDisableMFA))

	// Recovery codes (requires verified email)
	s.mux.HandleFunc("GET /api/v1/profile/mfa/recovery-codes", s.verifiedMiddleware(s.handleRecoveryCodesCount))
	s.mux.HandleFunc("POST /api/v1/profile/mfa/recovery-codes/regenerate", s.verifiedMiddleware(s.handleRegenerateRecoveryCodes))

	// Sessions — accessible without verified email (can still manage sessions)
	s.mux.HandleFunc("GET /api/v1/profile/sessions", s.authMiddleware(s.handleListSessions))
	s.mux.HandleFunc("DELETE /api/v1/profile/sessions/{id}", s.authMiddleware(s.handleRevokeSession))
	s.mux.HandleFunc("DELETE /api/v1/profile/sessions", s.authMiddleware(s.handleRevokeAllSessions))

	// ─── Orgs (requires verified email) ──────────────────────────────
	s.mux.HandleFunc("POST /api/v1/orgs", s.verifiedMiddleware(s.handleCreateOrg))
	s.mux.HandleFunc("GET /api/v1/orgs", s.verifiedMiddleware(s.handleListOrgs))
	s.mux.HandleFunc("GET /api/v1/orgs/{slug}", s.verifiedMiddleware(s.handleGetOrg))

	// ─── Feature flags (authenticated) ──────────────────────────────
	s.mux.HandleFunc("GET /api/v1/config/features", s.authMiddleware(s.handleListFeatureFlags))

	// ─── Tenant-scoped routes (auth + verified email + org context) ──
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
