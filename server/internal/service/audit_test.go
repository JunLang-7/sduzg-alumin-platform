package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/repository"
)

type fakeAuditStore struct {
	items       []repository.AuditEntry
	total       int64
	listErr     error
	detail      *repository.AuditEntry
	detailErr   error
	listQuery   do.AuditQuery
	detailQuery uint64
}

func (s *fakeAuditStore) List(_ context.Context, query do.AuditQuery) ([]repository.AuditEntry, int64, error) {
	s.listQuery = query
	return s.items, s.total, s.listErr
}

func (s *fakeAuditStore) GetByID(_ context.Context, id uint64) (*repository.AuditEntry, error) {
	s.detailQuery = id
	return s.detail, s.detailErr
}

func TestMapAuditChangeMasksSensitiveValues(t *testing.T) {
	change := mapAuditChange(AuditChange{
		FieldName:  "mobile",
		FieldLabel: "手机号",
		OldValue:   "13812345678",
		NewValue:   "13987654321",
	})
	if !change.Sensitive {
		t.Fatal("expected mobile change to be marked sensitive")
	}
	if change.OldValue != "已填写" || change.NewValue != "已修改" || change.CurrentValue != "已修改" {
		t.Fatalf("unexpected masked values: %+v", change)
	}
}

func TestMapAuditChangePreservesNonSensitiveValues(t *testing.T) {
	change := mapAuditChange(AuditChange{
		FieldName:    "work_unit",
		FieldLabel:   "工作单位",
		OldValue:     "山东某单位",
		NewValue:     "济南某单位",
		CurrentValue: "济南某单位",
	})
	if change.Sensitive {
		t.Fatal("did not expect work unit change to be marked sensitive")
	}
	if change.OldValue != "山东某单位" || change.NewValue != "济南某单位" {
		t.Fatalf("unexpected non-sensitive values: %+v", change)
	}
}

func TestAuditServiceListPassesFiltersAndMapsOperation(t *testing.T) {
	operatorName := "张三"
	targetName := "王海宁"
	targetID := uint64(88)
	createdAt := time.Date(2026, 9, 5, 10, 21, 0, 0, time.UTC)
	detail := `{"schema_version":1,"operator_name":"张三","operator_role_label":"子管理员","target_name":"王海宁","target_meta":"2020级","management_scope":"MPA专业学位研究生","source":"admin","status":"applied"}`
	store := &fakeAuditStore{
		items: []repository.AuditEntry{{
			ID:           17,
			OperatorRole: common.RoleAdmin,
			Action:       AuditActionUpdate,
			TargetType:   "alumni_profile",
			TargetID:     &targetID,
			Detail:       &detail,
			CreatedAt:    createdAt,
			OperatorName: &operatorName,
			TargetName:   &targetName,
		}},
		total: 1,
	}

	pager, err := NewAuditService(store).List(context.Background(), dto.AuditListRequest{
		Page:            2,
		PageSize:        10,
		StartDate:       "2026-09-01",
		EndDate:         "2026-09-05",
		ManagementScope: " MPA专业学位研究生 ",
		Action:          " update ",
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if store.listQuery.Action != AuditActionUpdate || store.listQuery.ManagementScope != "MPA专业学位研究生" {
		t.Fatalf("unexpected normalized filters: %+v", store.listQuery)
	}
	if store.listQuery.StartAt == nil || store.listQuery.EndAt == nil {
		t.Fatalf("expected date range in query: %+v", store.listQuery)
	}
	if len(pager.Items) != 1 || pager.Total != 1 {
		t.Fatalf("unexpected pager: %+v", pager)
	}
	item := pager.Items[0]
	if item.ID != 17 || item.Operator != "张三" || item.OperatorRoleLabel != "子管理员" || item.TargetName != "王海宁" || item.Action != AuditActionUpdate {
		t.Fatalf("unexpected mapped operation: %+v", item)
	}
	if len(item.Changes) != 0 {
		t.Fatalf("list item should not include field diff: %+v", item.Changes)
	}
}

func TestAuditServiceDetailMapsAndMasksFieldDiff(t *testing.T) {
	targetID := uint64(88)
	detail := `{"schema_version":1,"operator_name":"李四","operator_role_label":"子管理员","target_name":"王海宁","management_scope":"本科生","source":"admin","changes":[{"field_name":"work_unit","field_label":"工作单位","old_value":"旧单位","new_value":"新单位","current_value":"新单位"},{"field_name":"mobile","field_label":"手机号","old_value":"13800000000","new_value":"13900000000"}]}`
	store := &fakeAuditStore{
		detail: &repository.AuditEntry{ID: 19, Action: AuditActionUpdate, OperatorRole: common.RoleAdmin, TargetType: "alumni_profile", TargetID: &targetID, Detail: &detail},
	}

	item, err := NewAuditService(store).Detail(context.Background(), 19)
	if err != nil {
		t.Fatalf("Detail() error = %v", err)
	}
	if store.detailQuery != 19 || item.TargetName != "王海宁" || item.ManagementScope != "本科生" {
		t.Fatalf("unexpected detail header: %+v", item)
	}
	if len(item.Changes) != 2 {
		t.Fatalf("changes = %d, want 2", len(item.Changes))
	}
	if item.Changes[0].OldValue != "旧单位" || item.Changes[0].NewValue != "新单位" {
		t.Fatalf("non-sensitive diff was changed: %+v", item.Changes[0])
	}
	if !item.Changes[1].Sensitive || item.Changes[1].OldValue != "已填写" || item.Changes[1].NewValue != "已修改" {
		t.Fatalf("sensitive diff was not masked: %+v", item.Changes[1])
	}
}

func TestAuditServiceDetailPropagatesNotFound(t *testing.T) {
	store := &fakeAuditStore{detailErr: common.ErrAuditNotFound}
	_, err := NewAuditService(store).Detail(context.Background(), 404)
	if !errors.Is(err, common.ErrAuditNotFound) {
		t.Fatalf("Detail() error = %v, want ErrAuditNotFound", err)
	}
}
