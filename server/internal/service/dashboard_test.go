package service

import (
	"context"
	"errors"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
)

type fakeDashboardStore struct {
	stats             do.DashboardOverviewStats
	err               error
	distributionQuery do.DashboardDistributionQuery
	distributionItems []do.DashboardDistributionItem
	distributionErr   error
}

func (s *fakeDashboardStore) Overview(_ context.Context) (do.DashboardOverviewStats, error) {
	return s.stats, s.err
}

func (s *fakeDashboardStore) Distribution(_ context.Context, query do.DashboardDistributionQuery) ([]do.DashboardDistributionItem, error) {
	s.distributionQuery = query
	return s.distributionItems, s.distributionErr
}

func TestDashboardServiceOverviewCalculatesCompletionRates(t *testing.T) {
	store := &fakeDashboardStore{
		stats: do.DashboardOverviewStats{
			TotalAlumni:      4,
			TotalAccounts:    3,
			MobileComplete:   2,
			WorkUnitComplete: 1,
			MentorComplete:   0,
		},
	}
	svc := NewDashboardService(store)

	result, err := svc.Overview(context.Background())
	if err != nil {
		t.Fatalf("expected overview success, got %v", err)
	}
	if result.TotalAlumni != 4 || result.TotalAccounts != 3 {
		t.Fatalf("unexpected overview totals: %+v", result)
	}
	if result.MobileCompleteRate != 0.5 {
		t.Fatalf("expected mobile complete rate 0.5, got %v", result.MobileCompleteRate)
	}
	if result.WorkUnitCompleteRate != 0.25 {
		t.Fatalf("expected work unit complete rate 0.25, got %v", result.WorkUnitCompleteRate)
	}
	if result.MentorCompleteRate != 0 {
		t.Fatalf("expected mentor complete rate 0, got %v", result.MentorCompleteRate)
	}
}

func TestDashboardServiceOverviewHandlesEmptyAlumni(t *testing.T) {
	store := &fakeDashboardStore{
		stats: do.DashboardOverviewStats{
			TotalAlumni:      0,
			TotalAccounts:    0,
			MobileComplete:   3,
			WorkUnitComplete: 2,
			MentorComplete:   1,
		},
	}
	svc := NewDashboardService(store)

	result, err := svc.Overview(context.Background())
	if err != nil {
		t.Fatalf("expected overview success, got %v", err)
	}
	if result.MobileCompleteRate != 0 || result.WorkUnitCompleteRate != 0 || result.MentorCompleteRate != 0 {
		t.Fatalf("expected zero rates for empty alumni total, got %+v", result)
	}
}

func TestDashboardServiceOverviewReturnsDatabaseUnavailable(t *testing.T) {
	svc := NewDashboardService(&fakeDashboardStore{err: common.ErrDatabaseUnavailable})

	_, err := svc.Overview(context.Background())
	if !errors.Is(err, common.ErrDatabaseUnavailable) {
		t.Fatalf("expected database unavailable, got %v", err)
	}
}

func TestDashboardServiceDistributionNormalizesAndMapsItems(t *testing.T) {
	store := &fakeDashboardStore{
		distributionItems: []do.DashboardDistributionItem{
			{Name: "2020级", Value: 12},
			{Name: "未填", Value: 3},
		},
	}
	svc := NewDashboardService(store)

	items, err := svc.Distribution(context.Background(), dto.DashboardDistributionRequest{
		Dimension: " Grade ",
	})
	if err != nil {
		t.Fatalf("expected distribution success, got %v", err)
	}
	if store.distributionQuery.Dimension != do.DashboardDistributionDimensionGrade {
		t.Fatalf("expected normalized dimension grade, got %q", store.distributionQuery.Dimension)
	}
	if len(items) != 2 || items[0].Name != "2020级" || items[0].Value != 12 || items[1].Name != "未填" || items[1].Value != 3 {
		t.Fatalf("unexpected distribution items: %+v", items)
	}
}

func TestDashboardServiceDistributionRejectsInvalidDimension(t *testing.T) {
	store := &fakeDashboardStore{}
	svc := NewDashboardService(store)

	_, err := svc.Distribution(context.Background(), dto.DashboardDistributionRequest{
		Dimension: "name",
	})
	if !errors.Is(err, common.ErrInvalidRequest) {
		t.Fatalf("expected invalid request, got %v", err)
	}
	if store.distributionQuery.Dimension != "" {
		t.Fatalf("expected repository not to be called, got query %+v", store.distributionQuery)
	}
}

func TestDashboardServiceDistributionReturnsDatabaseUnavailable(t *testing.T) {
	svc := NewDashboardService(&fakeDashboardStore{distributionErr: common.ErrDatabaseUnavailable})

	_, err := svc.Distribution(context.Background(), dto.DashboardDistributionRequest{
		Dimension: do.DashboardDistributionDimensionIndustry,
	})
	if !errors.Is(err, common.ErrDatabaseUnavailable) {
		t.Fatalf("expected database unavailable, got %v", err)
	}
}
