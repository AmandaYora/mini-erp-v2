package apperror

// Code is the machine-readable error category. The user-facing sentence is
// always Message (Indonesian) — the frontend must display it, never discard it.
type Code string

const (
	CodeValidation   Code = "validation_failed"
	CodeUnauthorized Code = "unauthorized"
	CodeForbidden    Code = "forbidden"
	CodeNotFound     Code = "not_found"
	CodeConflict     Code = "conflict"
	CodeRateLimited  Code = "rate_limited"
	CodeInternal     Code = "internal_error"
)

// FieldError describes one rejected payload field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// AppError is the only error type handlers translate to HTTP responses.
// Err carries the internal cause for logs and is never serialized.
type AppError struct {
	Code    Code
	Message string
	Fields  []FieldError
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return string(e.Code) + ": " + e.Err.Error()
	}
	return string(e.Code) + ": " + e.Message
}

// HTTPStatus maps an error code to its HTTP status.
func HTTPStatus(c Code) int {
	switch c {
	case CodeValidation:
		return 400
	case CodeUnauthorized:
		return 401
	case CodeForbidden:
		return 403
	case CodeNotFound:
		return 404
	case CodeConflict:
		return 409
	case CodeRateLimited:
		return 429
	default:
		return 500
	}
}

// Validation reports payload problems. Fields may be empty only when the
// message alone explains the failure.
func Validation(msg string, fields []FieldError) *AppError {
	if msg == "" {
		msg = "Validasi gagal"
	}
	return &AppError{Code: CodeValidation, Message: msg, Fields: fields}
}

// Unauthorized is for missing/invalid/expired sessions.
func Unauthorized() *AppError {
	return &AppError{Code: CodeUnauthorized, Message: "Sesi berakhir, silakan login kembali"}
}

// Forbidden is the fail-closed default: no permission, no access.
func Forbidden() *AppError {
	return &AppError{Code: CodeForbidden, Message: "Anda tidak memiliki akses"}
}

// NotFound names the missing resource in Indonesian, e.g. NotFound("Cabang").
func NotFound(resource string) *AppError {
	if resource == "" {
		resource = "Data"
	}
	return &AppError{Code: CodeNotFound, Message: resource + " tidak ditemukan"}
}

// Conflict is for duplicate keys and state clashes. The message must name
// the conflicting value, e.g. "Kode cabang 'BLR' sudah digunakan".
func Conflict(msg string) *AppError {
	return &AppError{Code: CodeConflict, Message: msg}
}

// RateLimited is for throttled endpoints (login). Callers should retry
// after a short wait.
func RateLimited() *AppError {
	return &AppError{Code: CodeRateLimited, Message: "Terlalu banyak percobaan, coba lagi sebentar"}
}

// Internal hides the cause from clients. Log e.Err at the call site.
func Internal(err error) *AppError {
	return &AppError{Code: CodeInternal, Message: "Terjadi kesalahan internal", Err: err}
}
