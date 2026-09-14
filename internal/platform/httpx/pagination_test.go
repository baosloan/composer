package httpx

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPageQueryNormalize(t *testing.T) {
	tests := []struct {
		name             string
		in               PageQuery
		wantPage, wantSz int
	}{
		{"zero values get defaults", PageQuery{}, 1, 20},
		{"negatives get defaults", PageQuery{Page: -5, PageSize: -1}, 1, 20},
		{"valid values pass through", PageQuery{Page: 3, PageSize: 50}, 3, 50},
		{"oversized size is clamped", PageQuery{Page: 1, PageSize: 10000}, 1, 100},
		{"size at the cap is kept", PageQuery{Page: 1, PageSize: 100}, 1, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := tt.in
			q.Normalize()
			if q.Page != tt.wantPage || q.PageSize != tt.wantSz {
				t.Errorf("Normalize() = %d/%d, want %d/%d",
					q.Page, q.PageSize, tt.wantPage, tt.wantSz)
			}
		})
	}
}

func TestPageQueryOffset(t *testing.T) {
	tests := []struct {
		q    PageQuery
		want int
	}{
		{PageQuery{Page: 1, PageSize: 20}, 0},
		{PageQuery{Page: 2, PageSize: 20}, 20},
		{PageQuery{Page: 4, PageSize: 15}, 45},
	}
	for _, tt := range tests {
		if got := tt.q.Offset(); got != tt.want {
			t.Errorf("Offset() = %d, want %d", got, tt.want)
		}
	}
}

// TestSortClauseRejectsUnknownColumns is a security test, not an ergonomics
// one: `sort` arrives from the query string, so passing it to the database
// unchecked is a SQL injection.
func TestSortClauseRejectsUnknownColumns(t *testing.T) {
	allowed := []string{"id", "username", "created_at"}

	tests := []struct {
		name string
		sort string
		want string
	}{
		{"empty falls back", "", "id DESC"},
		{"whitelisted ascending", "username", "username ASC"},
		{"whitelisted descending", "-created_at", "created_at DESC"},
		{"unknown column falls back", "password", "id DESC"},
		{"injection attempt falls back", "id; DROP TABLE users--", "id DESC"},
		{"subquery attempt falls back", "(SELECT 1)", "id DESC"},
		{"unknown descending falls back", "-secret_column", "id DESC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := PageQuery{Sort: tt.sort}
			if got := q.SortClause("id DESC", allowed...); got != tt.want {
				t.Errorf("SortClause(%q) = %q, want %q", tt.sort, got, tt.want)
			}
		})
	}
}

func TestNewPageDerivesTotalPages(t *testing.T) {
	tests := []struct {
		total          int64
		pageSize, page int
		wantPages      int
		wantHasNext    bool
	}{
		{0, 20, 1, 0, false},
		{1, 20, 1, 1, false},
		{20, 20, 1, 1, false},
		{21, 20, 1, 2, true},   // ceiling division, not floor
		{100, 20, 5, 5, false}, // last page
		{100, 20, 3, 5, true},
	}

	for _, tt := range tests {
		q := PageQuery{Page: tt.page, PageSize: tt.pageSize}
		got := NewPage(q, tt.total, []string{})

		if got.TotalPages != tt.wantPages {
			t.Errorf("total=%d size=%d: TotalPages = %d, want %d",
				tt.total, tt.pageSize, got.TotalPages, tt.wantPages)
		}
		if got.HasNext != tt.wantHasNext {
			t.Errorf("total=%d page=%d: HasNext = %v, want %v",
				tt.total, tt.page, got.HasNext, tt.wantHasNext)
		}
	}
}

// TestNewPageSerializesEmptyItemsAsArray guards a contract clients depend on:
// a null where an array was promised is a classic front-end crash.
func TestNewPageSerializesEmptyItemsAsArray(t *testing.T) {
	page := NewPage[string](PageQuery{Page: 1, PageSize: 20}, 0, nil)

	encoded, err := json.Marshal(page)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(encoded), `"items":[]`) {
		t.Errorf("items serialized as null, not []: %s", encoded)
	}
}

func TestMapPagePreservesMetadata(t *testing.T) {
	src := NewPage(PageQuery{Page: 2, PageSize: 10}, 42, []int{1, 2, 3})

	dst := MapPage(src, func(i int) string { return string(rune('a' + i - 1)) })

	if dst.Total != 42 || dst.Page != 2 || dst.PageSize != 10 || dst.TotalPages != 5 {
		t.Errorf("metadata lost: %+v", dst)
	}
	if len(dst.Items) != 3 || dst.Items[0] != "a" {
		t.Errorf("items = %v, want [a b c]", dst.Items)
	}
}

// TestBodyAlwaysIncludesDataKey pins the envelope contract: the previous
// scaffold used omitempty on Data, which silently dropped the key for a nil
// payload and broke clients that trusted it to be there.
func TestBodyAlwaysIncludesDataKey(t *testing.T) {
	encoded, err := json.Marshal(Response{Code: 0, Message: "success", Data: nil})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(encoded), `"data":null`) {
		t.Errorf("data key was dropped for a nil payload: %s", encoded)
	}
}
