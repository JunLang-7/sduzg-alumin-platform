package service

import (
	"context"
	"testing"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
)

type fakeAlumniStore struct {
	query do.AlumniListQuery
	items []*model.AlumniProfile
	total int64
	err   error
}

func (s *fakeAlumniStore) List(_ context.Context, query do.AlumniListQuery) ([]*model.AlumniProfile, int64, error) {
	s.query = query
	return s.items, s.total, s.err
}

func TestAlumniServiceListNormalizesAndMapsItems(t *testing.T) {
	workUnit := "山东大学"
	position := "主任"
	updatedAt := time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC)
	store := &fakeAlumniStore{
		items: []*model.AlumniProfile{
			{
				ID:        9,
				Name:      "张三",
				Grade:     "2020级",
				WorkUnit:  &workUnit,
				Position:  &position,
				UpdatedAt: updatedAt,
			},
		},
		total: 12,
	}
	svc := NewAlumniService(store)

	pager, err := svc.List(context.Background(), dto.AlumniListRequest{
		Page:     0,
		PageSize: 1000,
		Keyword:  " 张三 ",
	})
	if err != nil {
		t.Fatalf("expected list success, got %v", err)
	}
	if store.query.Page.Page != common.DefaultPage || store.query.Page.PageSize != common.MaxPageSize {
		t.Fatalf("expected normalized page query, got %+v", store.query.Page)
	}
	if store.query.Keyword != "张三" {
		t.Fatalf("expected trimmed keyword, got %q", store.query.Keyword)
	}
	if pager.Page != common.DefaultPage || pager.PageSize != common.MaxPageSize || pager.Total != 12 {
		t.Fatalf("unexpected pager metadata: %+v", pager)
	}
	if len(pager.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(pager.Items))
	}
	if pager.Items[0].ID != 9 || pager.Items[0].Name != "张三" || pager.Items[0].WorkUnit == nil || *pager.Items[0].WorkUnit != workUnit {
		t.Fatalf("unexpected alumni item: %+v", pager.Items[0])
	}
}
