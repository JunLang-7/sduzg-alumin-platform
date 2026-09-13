package dto

import (
	"testing"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
)

func TestAuditListRequestToQueryUsesInclusiveEndDate(t *testing.T) {
	query, err := (AuditListRequest{
		Page:            2,
		PageSize:        50,
		StartDate:       "2026-09-01",
		EndDate:         "2026-09-07",
		ManagementScope: "指定年级",
		Action:          "update",
		TargetID:        1024,
	}).ToQuery()
	if err != nil {
		t.Fatalf("expected valid query, got %v", err)
	}
	if query.Page.Page != 2 || query.Page.PageSize != 50 {
		t.Fatalf("unexpected pagination: %+v", query.Page)
	}
	if query.StartAt == nil || !query.StartAt.Equal(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected start boundary: %v", query.StartAt)
	}
	if query.EndAt == nil || !query.EndAt.Equal(time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected end boundary: %v", query.EndAt)
	}
	if query.TargetID == nil || *query.TargetID != 1024 {
		t.Fatalf("unexpected target id: %v", query.TargetID)
	}
}

func TestAuditListRequestToQueryRejectsInvalidDate(t *testing.T) {
	_, err := (AuditListRequest{StartDate: "2026/09/01"}).ToQuery()
	if err != common.ErrInvalidRequest {
		t.Fatalf("expected invalid request, got %v", err)
	}
}
