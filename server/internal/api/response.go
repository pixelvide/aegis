package api

import (
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"strconv"

	"github.com/pixelvide/aegis/server/internal/requestid"
)

// ─── Response Types ─────────────────────────────────────────────────────────

// ResultInfo holds pagination metadata for list responses (Cloudflare-style).
type ResultInfo struct {
	Page       int  `json:"page"`
	PerPage    int  `json:"per_page"`
	Total      int  `json:"total"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

// NewResultInfo creates a ResultInfo from page, perPage, and total count.
func NewResultInfo(page, perPage, total int) ResultInfo {
	totalPages := 0
	if perPage > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(perPage)))
	}
	return ResultInfo{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}
}

// ApiError represents a structured error in the Cloudflare-style error array.
type ApiError struct {
	Type    string         `json:"type"`
	Code    string         `json:"code"`
	Ref     string         `json:"ref"`
	Status  int            `json:"-"` // HTTP status code (not serialized in JSON)
	Message string         `json:"message"`
	Details []FieldError   `json:"details,omitempty"`
}

// FieldError represents a field-level validation error.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// WithMessage returns a copy of the ApiError with a custom message override.
func (e ApiError) WithMessage(msg string) ApiError {
	e.Message = msg
	return e
}

// WithDetails returns a copy of the ApiError with field-level validation details.
func (e ApiError) WithDetails(details ...FieldError) ApiError {
	e.Details = details
	return e
}

// ─── Envelope Structs ───────────────────────────────────────────────────────

// successEnvelope is the base structure for all success responses.
type successEnvelope struct {
	Success   bool   `json:"success"`
	RequestID string `json:"request_id,omitempty"`
}

// resultEnvelope wraps a single resource.
type resultEnvelope struct {
	successEnvelope
	Result  any    `json:"result"`
	Message string `json:"message,omitempty"`
}

// listEnvelope wraps a paginated list.
type listEnvelope struct {
	successEnvelope
	Result     any        `json:"result"`
	ResultInfo ResultInfo `json:"result_info"`
}

// errorEnvelope wraps one or more errors.
type errorEnvelope struct {
	Success   bool       `json:"success"`
	RequestID string     `json:"request_id,omitempty"`
	Errors    []ApiError `json:"errors"`
}

// ─── Response Writers ───────────────────────────────────────────────────────

// writeResult writes a single resource wrapped in the standard envelope.
//
//	{"success": true, "request_id": "req_...", "result": {...}}
func writeResult(w http.ResponseWriter, r *http.Request, status int, result any) {
	env := resultEnvelope{
		successEnvelope: successEnvelope{
			Success:   true,
			RequestID: requestid.FromContext(r.Context()),
		},
		Result: result,
	}
	writeEnvelope(w, status, env)
}

// writeResultMessage writes a single resource with a confirmation message.
//
//	{"success": true, "request_id": "req_...", "result": {...}, "message": "..."}
func writeResultMessage(w http.ResponseWriter, r *http.Request, status int, result any, msg string) {
	env := resultEnvelope{
		successEnvelope: successEnvelope{
			Success:   true,
			RequestID: requestid.FromContext(r.Context()),
		},
		Result:  result,
		Message: msg,
	}
	writeEnvelope(w, status, env)
}

// writeList writes a paginated list wrapped in the standard envelope.
//
//	{"success": true, "request_id": "req_...", "result": [...], "result_info": {...}}
func writeList(w http.ResponseWriter, r *http.Request, result any, resultInfo ResultInfo) {
	env := listEnvelope{
		successEnvelope: successEnvelope{
			Success:   true,
			RequestID: requestid.FromContext(r.Context()),
		},
		Result:     result,
		ResultInfo: resultInfo,
	}
	writeEnvelope(w, http.StatusOK, env)
}

// writeMessage writes a message-only success response (e.g., logout, password reset).
//
//	{"success": true, "request_id": "req_...", "message": "..."}
func writeMessage(w http.ResponseWriter, r *http.Request, status int, msg string) {
	env := resultEnvelope{
		successEnvelope: successEnvelope{
			Success:   true,
			RequestID: requestid.FromContext(r.Context()),
		},
		Message: msg,
	}
	writeEnvelope(w, status, env)
}

// writeApiError writes one or more structured errors in the Cloudflare-style envelope.
//
//	{"success": false, "request_id": "req_...", "errors": [{...}]}
//
// The HTTP status code is taken from the first error in the list.
func writeApiError(w http.ResponseWriter, r *http.Request, errs ...ApiError) {
	if len(errs) == 0 {
		return
	}
	env := errorEnvelope{
		Success:   false,
		RequestID: requestid.FromContext(r.Context()),
		Errors:    errs,
	}
	writeEnvelope(w, errs[0].Status, env)
}

// writeValidationErrors writes field-level validation errors as a structured error response.
func writeValidationErrors(w http.ResponseWriter, r *http.Request, details ...FieldError) {
	err := ApiError{
		Type:    "validation_error",
		Code:    "invalid_request",
		Ref:     "E50001",
		Status:  http.StatusBadRequest,
		Message: "Invalid request",
		Details: details,
	}
	writeApiError(w, r, err)
}

// ─── Pagination Parsing ─────────────────────────────────────────────────────

// parseResultInfo extracts page and per_page from query parameters.
// Defaults: page=1, per_page=25. Clamps per_page to [1, 100].
func parseResultInfo(r *http.Request) (page, perPage int) {
	page = 1
	perPage = 25

	if v := r.URL.Query().Get("page"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			page = p
		}
	}

	if v := r.URL.Query().Get("per_page"); v != "" {
		if pp, err := strconv.Atoi(v); err == nil {
			perPage = pp
		}
	}

	// Clamp per_page to [1, 100]
	if perPage < 1 {
		perPage = 1
	}
	if perPage > 100 {
		perPage = 100
	}

	return page, perPage
}

// ─── Internal ───────────────────────────────────────────────────────────────

// writeEnvelope is the internal JSON serializer for all envelope types.
func writeEnvelope(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("json encode error", "error", err)
	}
}
