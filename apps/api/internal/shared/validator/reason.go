package validator

import (
	"strings"
	"unicode/utf8"

	"mini-erp/internal/shared/apperror"
)

// ReasonMaxRunes caps a free-text correction reason (the cancel_reason
// columns are varchar(255)).
const ReasonMaxRunes = 255

// Reason trims a free-text reason and checks it: mandatory when required, at
// most ReasonMaxRunes characters. A failure is a validation error on field
// "reason"; the returned error is a plain nil when the reason is fine.
func Reason(s string, required bool) (string, error) {
	s = strings.TrimSpace(s)
	if required && s == "" {
		return "", apperror.Validation("", []apperror.FieldError{{Field: "reason", Message: "alasan wajib diisi"}})
	}
	if utf8.RuneCountInString(s) > ReasonMaxRunes {
		return "", apperror.Validation("", []apperror.FieldError{{Field: "reason", Message: "maksimal 255 karakter"}})
	}
	return s, nil
}
