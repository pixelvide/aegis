package api

import "net/http"

func (s *Server) handleDashboardStats(w http.ResponseWriter, r *http.Request) {
	stats, err := tenantStore(r).GetDashboardStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get dashboard stats")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}
