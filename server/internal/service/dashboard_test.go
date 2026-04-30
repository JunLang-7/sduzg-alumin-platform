package service

import (
	"context"
	"errors"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
)

type fakeDashboardStore struct {
	stats do.DashboardOverviewStats
	err   error
}

func (s *fakeDashboardStore) Overview(_ context.Context) (do.DashboardOverviewStats, error) {
	return s.stats, s.err
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
