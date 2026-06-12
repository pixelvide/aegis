package api

import "net/http"

func (s *Server) handleDashboardStats(w http.ResponseWriter, r *http.Request) {
	stats, err := tenantStore(r).GetDashboardStats(r.Context(), "")
	if err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}
	writeResult(w, r, http.StatusOK, stats)
}

func (s *Server) handleProjectDashboardStats(w http.ResponseWriter, r *http.Request) {
	projectId := pathParam(r, "projectId")
	stats, err := tenantStore(r).GetDashboardStats(r.Context(), projectId)
	if err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}
	writeResult(w, r, http.StatusOK, stats)
}
