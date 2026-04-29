package common

import "testing"

func TestPageQueryNormalize(t *testing.T) {
	query := PageQuery{Page: -1, PageSize: 1000}.Normalize()
	if query.Page != DefaultPage {
		t.Fatalf("expected default page %d, got %d", DefaultPage, query.Page)
	}
	if query.PageSize != MaxPageSize {
		t.Fatalf("expected max page size %d, got %d", MaxPageSize, query.PageSize)
	}
}

func TestNewPagerUsesEmptyItems(t *testing.T) {
	pager := NewPager[string](nil, PageQuery{}, 3)
	if pager.Items == nil {
		t.Fatal("expected empty items slice, got nil")
	}
	if pager.Page != DefaultPage || pager.PageSize != DefaultPageSize {
		t.Fatalf("expected default pagination, got page=%d page_size=%d", pager.Page, pager.PageSize)
	}
	if pager.Total != 3 {
		t.Fatalf("expected total 3, got %d", pager.Total)
	}
}
