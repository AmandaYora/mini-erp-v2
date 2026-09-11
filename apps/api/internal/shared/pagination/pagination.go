package pagination

import "strconv"

const (
	// DefaultLimit is used when the client sends a missing, non-numeric,
	// zero, or negative limit (KI-26 class: never an empty list, never a crash).
	DefaultLimit = 20
	// MaxLimit caps runaway page sizes.
	MaxLimit = 100
	// DefaultPage is the first page.
	DefaultPage = 1
)

// Parse sanitizes raw page/limit query values. It never returns zero or
// negative numbers and never errors — every abnormal input falls back to a
// safe default instead of producing an empty list or a 500.
func Parse(pageRaw, limitRaw string) (page, limit int) {
	page = DefaultPage
	limit = DefaultLimit
	if p, err := strconv.Atoi(pageRaw); err == nil && p >= 1 {
		page = p
	}
	if l, err := strconv.Atoi(limitRaw); err == nil && l >= 1 {
		limit = l
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	return page, limit
}

// Offset converts 1-based page + limit to a SQL offset.
func Offset(page, limit int) int {
	return (page - 1) * limit
}

// TotalPages rounds the page count up. Zero items still yields zero pages.
func TotalPages(total int64, limit int) int {
	if limit <= 0 || total <= 0 {
		return 0
	}
	return int((total + int64(limit) - 1) / int64(limit))
}
