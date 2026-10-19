package response

import (
	"encoding/json"
	"net/http"

	"mini-erp/internal/shared/apperror"
)

// Meta carries pagination info. JSON key is `meta` (see API_CONTRACT).
type Meta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
}

type successBody struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data"`
	Meta    *Meta  `json:"meta,omitempty"`
}

type errorBody struct {
	Success bool                  `json:"success"`
	Message string                `json:"message"`
	Code    string                `json:"code,omitempty"`
	Errors  []apperror.FieldError `json:"errors,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// OK writes {success:true,message,data} with HTTP 200.
func OK(w http.ResponseWriter, message string, data any) {
	writeJSON(w, http.StatusOK, successBody{Success: true, Message: message, Data: data})
}

// Created writes {success:true,message,data} with HTTP 201.
func Created(w http.ResponseWriter, message string, data any) {
	writeJSON(w, http.StatusCreated, successBody{Success: true, Message: message, Data: data})
}

// Paginated writes {success:true,message,data,meta} with HTTP 200.
func Paginated(w http.ResponseWriter, message string, data any, meta Meta) {
	writeJSON(w, http.StatusOK, successBody{Success: true, Message: message, Data: data, Meta: &meta})
}

// FailErr accepts any error: *apperror.AppError passes through, foreign
// errors become internal (never leaking). Handlers should prefer this over
// type-asserting service results.
func FailErr(w http.ResponseWriter, err error) {
	if e, ok := err.(*apperror.AppError); ok {
		Fail(w, e)
		return
	}
	Fail(w, apperror.Internal(err))
}

// Fail writes {success:false,message,errors?} with the status mapped from the error code.
func Fail(w http.ResponseWriter, err *apperror.AppError) {
	writeJSON(w, apperror.HTTPStatus(err.Code), errorBody{
		Success: false,
		Message: err.Message,
		Code:    string(err.Code),
		Errors:  err.Fields,
	})
}

// Unauthorized writes 401 with a machine-readable code (F1-10 refresh_raced).
func Unauthorized(w http.ResponseWriter, code, message string) {
	if code == "" {
		code = string(apperror.CodeUnauthorized)
	}
	writeJSON(w, http.StatusUnauthorized, errorBody{Success: false, Message: message, Code: code})
}

// Error writes {success:false,message} with an explicit status and no code or
// field errors. It backs the fixed-wording guards (session, permission,
// recover, readiness) so the envelope shape lives only in this package.
func Error(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorBody{Success: false, Message: message})
}

// SessionExpired writes the 401 used when no valid session accompanies a request.
func SessionExpired(w http.ResponseWriter) {
	Error(w, http.StatusUnauthorized, "Sesi berakhir, silakan login kembali")
}

// Forbidden writes the generic 403.
func Forbidden(w http.ResponseWriter) {
	ForbiddenMsg(w, "Anda tidak memiliki akses")
}

// ForbiddenMsg writes a 403 with a specific message.
func ForbiddenMsg(w http.ResponseWriter, message string) {
	Error(w, http.StatusForbidden, message)
}

// Internal writes the generic 500 without leaking details.
func Internal(w http.ResponseWriter) {
	Error(w, http.StatusInternalServerError, "Terjadi kesalahan internal")
}

// Unavailable writes a 503 with a message (service not ready).
func Unavailable(w http.ResponseWriter, message string) {
	Error(w, http.StatusServiceUnavailable, message)
}
