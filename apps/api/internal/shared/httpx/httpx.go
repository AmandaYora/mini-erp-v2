package httpx

import (
	"encoding/json"
	"mime/multipart"
	"net/http"
	"strconv"

	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/validator"
)

// DecodeJSON parses a JSON body (max 1 MB) into dst and runs `validate`
// tags, returning field errors in one pass. Every write endpoint uses this —
// payloads are never trusted on type alone (PRD §6.1).
func DecodeJSON(r *http.Request, dst any) []apperror.FieldError {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return []apperror.FieldError{{Field: "", Message: "Body JSON tidak valid"}}
	}
	return validator.Validate(dst)
}

// MultipartFile extracts the "file" part, capped at maxBytes.
func MultipartFile(r *http.Request, maxBytes int64) (multipart.File, *multipart.FileHeader, *apperror.AppError) {
	r.Body = http.MaxBytesReader(nil, r.Body, maxBytes)
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		return nil, nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: "gagal membaca upload"}})
	}
	fh, header, err := r.FormFile("file")
	if err != nil {
		return nil, nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: "wajib diisi"}})
	}
	return fh, header, nil
}

// PathInt extracts an integer path variable, e.g. PathInt(r, "id") for
// patterns like "/api/v1/users/{id}".
func PathInt(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.PathValue(name), 10, 64)
}
