package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
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

func (s *fakeAuditStore) GetByID(_ context.Context, id uint64, query do.AuditQuery) (*repository.AuditEntry, error) {
	s.detailQuery = id
	s.listQuery = query
	return s.detail, s.detailErr
}

type fakeAuditAlumniStore struct {
	*fakeAlumniStore
	profile      *model.AlumniProfile
	getDomainIDs []uint64
	err          error
}

func (s *fakeAuditAlumniStore) GetByID(_ context.Context, id uint64, dataDomainIDs []uint64) (*model.AlumniProfile, error) {
	s.getDomainIDs = append([]uint64(nil), dataDomainIDs...)
	if s.err != nil {
		return nil, s.err
	}
	if s.profile == nil || s.profile.ID != id {
		return nil, common.ErrAlumniNotFound
	}
	return s.profile, nil
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
	if !change.Sensitive {
		t.Fatal("expected work unit change to be marked sensitive")
	}
	if change.OldValue != "已填写" || change.NewValue != "已修改" || change.CurrentValue != "已修改" {
		t.Fatalf("unexpected masked values: %+v", change)
	}
}

func TestMapAuditChangeMasksAllSensitiveFields(t *testing.T) {
	for _, field := range []string{"mobile", "email", "work_unit", "position", "mailing_address"} {
		t.Run(field, func(t *testing.T) {
			change := mapAuditChange(AuditChange{
				FieldName:    field,
				OldValue:     "旧敏感值",
				NewValue:     "新敏感值",
				CurrentValue: "新敏感值",
			})
			if !change.Sensitive {
				t.Fatalf("field %q was not marked sensitive", field)
			}
			if change.OldValue != "已填写" || change.NewValue != "已修改" || change.CurrentValue != "已修改" {
				t.Fatalf("field %q was not masked: %+v", field, change)
			}
		})
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

	pager, err := NewAuditService(store).List(context.Background(), common.AccessContext{Role: common.RoleSuperAdmin}, dto.AuditListRequest{
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

	item, err := NewAuditService(store).Detail(context.Background(), common.AccessContext{Role: common.RoleSuperAdmin}, 19)
	if err != nil {
		t.Fatalf("Detail() error = %v", err)
	}
	if store.detailQuery != 19 || item.TargetName != "王海宁" || item.ManagementScope != "本科生" {
		t.Fatalf("unexpected detail header: %+v", item)
	}
	if len(item.Changes) != 2 {
		t.Fatalf("changes = %d, want 2", len(item.Changes))
	}
	if !item.Changes[0].Sensitive || item.Changes[0].OldValue != "已填写" || item.Changes[0].NewValue != "已修改" {
		t.Fatalf("work unit diff was not masked: %+v", item.Changes[0])
	}
	if !item.Changes[1].Sensitive || item.Changes[1].OldValue != "已填写" || item.Changes[1].NewValue != "已修改" {
		t.Fatalf("sensitive diff was not masked: %+v", item.Changes[1])
	}
}

func TestAuditServiceBatchDetailHidesSensitiveFieldsWithoutPermission(t *testing.T) {
	detail := `{"schema_version":1,"target_name":"校友档案批量导入","batch_created_alumni":[{"id":401,"name":"周七","target_meta":"2020级 · 公共管理","field_values":{"name":"周七","grade":"2020级","mobile":"13800000000","email":"secret@example.com","work_unit":"山东某单位","position":"主任","mailing_address":"济南市"}}],"batch_hidden_sensitive_count":5}`
	store := &fakeAuditStore{detail: &repository.AuditEntry{
		ID:         21,
		Action:     AuditActionImport,
		TargetType: auditTargetBatch,
		Detail:     &detail,
	}}

	item, err := NewAuditService(store).Detail(context.Background(), common.AccessContext{Role: common.RoleAdmin, DomainIDs: []uint64{2}}, 21)
	if err != nil {
		t.Fatalf("Detail() error = %v", err)
	}
	if len(item.BatchImportFields) != len(auditBatchPublicFields) {
		t.Fatalf("visible batch fields = %d, want %d: %+v", len(item.BatchImportFields), len(auditBatchPublicFields), item.BatchImportFields)
	}
	if len(item.BatchCreatedAlumni) != 1 {
		t.Fatalf("batch records = %d, want 1", len(item.BatchCreatedAlumni))
	}
	for _, field := range auditBatchSensitiveFields {
		if _, ok := item.BatchCreatedAlumni[0].FieldValues[field.FieldName]; ok {
			t.Fatalf("unauthorized detail exposed sensitive field %q: %+v", field.FieldName, item.BatchCreatedAlumni[0].FieldValues)
		}
	}
	if item.BatchHiddenSensitiveCount != len(auditBatchSensitiveFields) {
		t.Fatalf("hidden sensitive count = %d, want %d", item.BatchHiddenSensitiveCount, len(auditBatchSensitiveFields))
	}
}

func TestAuditServiceBatchDetailLoadsCompleteSensitiveFieldsWithPermission(t *testing.T) {
	mobile := "13800000000"
	email := "zhouqi@example.com"
	workUnit := "山东某单位"
	position := "主任"
	address := "济南市历下区"
	alumniStore := &fakeAuditAlumniStore{
		fakeAlumniStore: &fakeAlumniStore{},
		profile: &model.AlumniProfile{
			ID:             401,
			Name:           "周七",
			Grade:          "2020级",
			Mobile:         &mobile,
			Email:          &email,
			WorkUnit:       &workUnit,
			Position:       &position,
			MailingAddress: &address,
		},
	}
	detail := `{"schema_version":1,"target_name":"校友档案批量导入","batch_created_alumni":[{"id":401,"name":"周七","target_meta":"2020级 · 公共管理","field_values":{"name":"周七","grade":"2020级"}}],"batch_hidden_sensitive_count":5}`
	store := &fakeAuditStore{detail: &repository.AuditEntry{
		ID:         22,
		Action:     AuditActionImport,
		TargetType: auditTargetBatch,
		Detail:     &detail,
	}}
	access := common.AccessContext{
		Role:        common.RoleAdmin,
		DomainIDs:   []uint64{2},
		Permissions: map[string]bool{common.PermissionAlumniSensitiveRead: true},
	}

	item, err := NewAuditService(store, alumniStore).Detail(context.Background(), access, 22)
	if err != nil {
		t.Fatalf("Detail() error = %v", err)
	}
	if len(item.BatchImportFields) != len(auditBatchPublicFields)+len(auditBatchSensitiveFields) {
		t.Fatalf("authorized batch fields = %d, want %d: %+v", len(item.BatchImportFields), len(auditBatchPublicFields)+len(auditBatchSensitiveFields), item.BatchImportFields)
	}
	values := item.BatchCreatedAlumni[0].FieldValues
	for field, want := range map[string]string{
		"mobile":          mobile,
		"email":           email,
		"work_unit":       workUnit,
		"position":        position,
		"mailing_address": address,
	} {
		if values[field] != want {
			t.Fatalf("authorized field %q = %q, want %q; values=%+v", field, values[field], want, values)
		}
	}
	if len(alumniStore.getDomainIDs) != 1 || alumniStore.getDomainIDs[0] != 2 {
		t.Fatalf("authorized lookup used unexpected domains: %+v", alumniStore.getDomainIDs)
	}
}

func TestAuditServiceDetailPropagatesNotFound(t *testing.T) {
	store := &fakeAuditStore{detailErr: common.ErrAuditNotFound}
	_, err := NewAuditService(store).Detail(context.Background(), common.AccessContext{Role: common.RoleSuperAdmin}, 404)
	if !errors.Is(err, common.ErrAuditNotFound) {
		t.Fatalf("Detail() error = %v, want ErrAuditNotFound", err)
	}
}

func TestAuditServiceRestrictsAdminHistoryToAssignedDomains(t *testing.T) {
	store := &fakeAuditStore{}
	access := common.AccessContext{Role: common.RoleAdmin, DomainIDs: []uint64{2, 7}}

	_, err := NewAuditService(store).List(context.Background(), access, dto.AuditListRequest{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if !store.listQuery.RestrictToDataDomains {
		t.Fatal("expected admin audit query to be restricted to data domains")
	}
	if len(store.listQuery.DataDomainIDs) != 2 || store.listQuery.DataDomainIDs[0] != 2 || store.listQuery.DataDomainIDs[1] != 7 {
		t.Fatalf("unexpected allowed domains: %+v", store.listQuery.DataDomainIDs)
	}
}

func TestAuditServiceAllowsSuperAdminHistoryAcrossDomains(t *testing.T) {
	store := &fakeAuditStore{}

	_, err := NewAuditService(store).List(context.Background(), common.AccessContext{Role: common.RoleSuperAdmin}, dto.AuditListRequest{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if store.listQuery.RestrictToDataDomains {
		t.Fatal("super admin history should not be restricted to assigned domains")
	}
}
