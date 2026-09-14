package httpx

import (
	"slices"
	"strings"
)

const (
	defaultPage     = 1
	defaultPageSize = 20
	maxPageSize     = 100
)

// PageQuery is the pagination and sorting input, embedded into a module's list
// request struct:
//
//	type ListIn struct {
//	    httpx.PageQuery
//	    Keyword string `form:"keyword" binding:"omitempty,max=50"`
//	}
type PageQuery struct {
	Page     int `form:"page" json:"page" binding:"omitempty,min=1"`
	PageSize int `form:"page_size" json:"page_size" binding:"omitempty,min=1,max=100"`

	// Sort is a column name, optionally prefixed with '-' for descending
	// order, e.g. "-created_at". The column must be whitelisted by the
	// repository via SortClause; it is never interpolated into SQL directly.
	Sort string `form:"sort" json:"sort" binding:"omitempty,max=64"`
}

// Normalize clamps page and size into a legal range.
//
// It is called by the service layer rather than trusted from the request, so
// callers that build a PageQuery programmatically get the same bounds.
func (q *PageQuery) Normalize() {
	if q.Page <= 0 {
		q.Page = defaultPage
	}
	if q.PageSize <= 0 {
		q.PageSize = defaultPageSize
	}
	if q.PageSize > maxPageSize {
		q.PageSize = maxPageSize
	}
}

// Offset returns the SQL OFFSET for the current page.
func (q PageQuery) Offset() int { return (q.Page - 1) * q.PageSize }

// Limit returns the SQL LIMIT for the current page.
func (q PageQuery) Limit() int { return q.PageSize }

// SortClause converts the requested sort into a SQL ORDER BY fragment,
// accepting only columns present in allowed.
//
// This whitelist is the whole point: `sort` comes straight from the query
// string, so passing it to the database unchecked is a SQL injection. An
// unknown or empty column falls back to fallback.
func (q PageQuery) SortClause(fallback string, allowed ...string) string {
	col := strings.TrimSpace(q.Sort)
	if col == "" {
		return fallback
	}

	desc := strings.HasPrefix(col, "-")
	col = strings.TrimPrefix(col, "-")

	if !slices.Contains(allowed, col) {
		return fallback
	}
	if desc {
		return col + " DESC"
	}
	return col + " ASC"
}

// Page is a paginated payload.
type Page[T any] struct {
	Items    []T   `json:"items"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
	// TotalPages saves every client from recomputing the ceiling division.
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
}

// NewPage assembles a Page, deriving the computed fields.
//
// Items is normalized to an empty slice so the JSON is always [] and never
// null: a null here is a classic source of client-side crashes.
func NewPage[T any](q PageQuery, total int64, items []T) Page[T] {
	if items == nil {
		items = []T{}
	}
	totalPages := 0
	if q.PageSize > 0 {
		totalPages = int((total + int64(q.PageSize) - 1) / int64(q.PageSize))
	}
	return Page[T]{
		Items:      items,
		Page:       q.Page,
		PageSize:   q.PageSize,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    q.Page < totalPages,
	}
}

// MapPage converts a Page of one type into a Page of another, which is how a
// service turns a page of entities into a page of DTOs without re-deriving the
// pagination metadata.
func MapPage[S, D any](src Page[S], fn func(S) D) Page[D] {
	items := make([]D, 0, len(src.Items))
	for _, s := range src.Items {
		items = append(items, fn(s))
	}
	return Page[D]{
		Items:      items,
		Page:       src.Page,
		PageSize:   src.PageSize,
		Total:      src.Total,
		TotalPages: src.TotalPages,
		HasNext:    src.HasNext,
	}
}
