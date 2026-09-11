package validator

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"

	"mini-erp/internal/shared/apperror"
)

var validate = validator.New()

// indonesian messages per validation tag. Unknown tags fall back to the
// library message so no failure ever goes unexplained.
func message(fe validator.FieldError) string {
	p := fe.Param()
	switch fe.Tag() {
	case "required":
		return "wajib diisi"
	case "email":
		return "format email tidak valid"
	case "min":
		return fmt.Sprintf("minimal %s karakter", p)
	case "max":
		return fmt.Sprintf("maksimal %s karakter", p)
	case "len":
		return fmt.Sprintf("harus %s karakter", p)
	case "gte":
		return fmt.Sprintf("minimal %s", p)
	case "lte":
		return fmt.Sprintf("maksimal %s", p)
	case "gt":
		return fmt.Sprintf("harus lebih dari %s", p)
	case "lt":
		return fmt.Sprintf("harus kurang dari %s", p)
	case "oneof":
		return fmt.Sprintf("harus salah satu dari: %s", strings.ReplaceAll(p, " ", ", "))
	case "numeric":
		return "harus berupa angka"
	case "url":
		return "format URL tidak valid"
	default:
		return fe.Error()
	}
}

// Validate checks a payload struct against its `validate` tags and returns
// one FieldError per violation, or nil when valid.
func Validate(payload any) []apperror.FieldError {
	if err := validate.Struct(payload); err == nil {
		return nil
	} else if verrs, ok := err.(validator.ValidationErrors); ok {
		out := make([]apperror.FieldError, 0, len(verrs))
		typ := reflect.TypeOf(payload)
		for typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
		for _, fe := range verrs {
			name := fe.Field()
			if typ.Kind() == reflect.Struct {
				if sf, ok := typ.FieldByName(fe.StructField()); ok {
					if tag := sf.Tag.Get("json"); tag != "" && tag != "-" {
						name = strings.Split(tag, ",")[0]
					}
				}
			}
			out = append(out, apperror.FieldError{Field: name, Message: message(fe)})
		}
		return out
	}
	// Non-field error (e.g. nil payload): report as a single generic failure.
	return []apperror.FieldError{{Field: "", Message: "Payload tidak valid"}}
}
