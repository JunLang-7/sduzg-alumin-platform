package service

import (
	"context"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
)

type captureAuditWriter struct {
	events []AuditEvent
}

func (w *captureAuditWriter) WriteAudit(_ context.Context, event AuditEvent) error {
	w.events = append(w.events, event)
	return nil
}

func auditOperator() common.AccessContext {
	return common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin}
}

func auditOperatorUser() *model.User {
	name := "总管理员"
	return &model.User{ID: 7, Account: "root", Role: common.RoleSuperAdmin, RealName: &name}
}

func TestAlumniServiceCreateWritesCreateAudit(t *testing.T) {
	created := &model.AlumniProfile{ID: 101, Name: "李四", Grade: "2021级", Status: common.AlumniStatusActive}
	store := &fakeAlumniStore{createResult: created}
	users := &fakeUserStore{usersByID: map[uint64]*model.User{7: auditOperatorUser()}}
	writer := &captureAuditWriter{}

	_, err := NewAlumniService(store, nil).
		WithAuditUserStore(users).
		WithAuditWriter(writer).
		Create(context.Background(), auditOperator(), dto.AdminAlumniCreateRequest{Name: created.Name, Grade: created.Grade})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(writer.events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(writer.events))
	}
	event := writer.events[0]
	if event.Action != AuditActionCreate || event.Source != "admin" || event.TargetID == nil || *event.TargetID != created.ID {
		t.Fatalf("unexpected create audit event: %+v", event)
	}
	if len(event.Changes) == 0 || event.Changes[0].FieldName != "name" {
		t.Fatalf("create audit changes = %+v, want name diff", event.Changes)
	}
}

func TestAlumniServiceUpdateWritesOnlyChangedFields(t *testing.T) {
	oldUnit := "旧单位"
	newUnit := "新单位"
	before := &model.AlumniProfile{ID: 102, Name: "王五", Grade: "2020级", WorkUnit: &oldUnit, Status: common.AlumniStatusActive}
	after := &model.AlumniProfile{ID: 102, Name: "王五", Grade: "2020级", WorkUnit: &newUnit, Status: common.AlumniStatusActive}
	store := &fakeAlumniStore{detail: before, updateResult: after}
	users := &fakeUserStore{usersByID: map[uint64]*model.User{7: auditOperatorUser()}}
	writer := &captureAuditWriter{}

	_, err := NewAlumniService(store, nil).
		WithAuditUserStore(users).
		WithAuditWriter(writer).
		Update(context.Background(), auditOperator(), before.ID, dto.AdminAlumniUpdateRequest{Name: before.Name, Grade: before.Grade, WorkUnit: &newUnit})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if len(writer.events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(writer.events))
	}
	event := writer.events[0]
	if event.Action != AuditActionUpdate || len(event.Changes) != 1 || event.Changes[0].FieldName != "work_unit" {
		t.Fatalf("unexpected update audit event: %+v", event)
	}
}

func TestAlumniServiceDeleteWritesDeleteAudit(t *testing.T) {
	before := &model.AlumniProfile{ID: 103, Name: "赵六", Grade: "2019级", Status: common.AlumniStatusActive}
	store := &fakeAlumniStore{detail: before}
	users := &fakeUserStore{usersByID: map[uint64]*model.User{7: auditOperatorUser()}}
	writer := &captureAuditWriter{}

	err := NewAlumniService(store, nil).
		WithAuditUserStore(users).
		WithAuditWriter(writer).
		Delete(context.Background(), auditOperator(), before.ID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if len(writer.events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(writer.events))
	}
	event := writer.events[0]
	if event.Action != AuditActionDelete || len(event.Changes) != 1 || event.Changes[0].FieldName != "status" {
		t.Fatalf("unexpected delete audit event: %+v", event)
	}
}

func TestAlumniServiceUpdateMeWritesAlumniSelfAudit(t *testing.T) {
	oldUnit := "旧单位"
	newUnit := "新单位"
	alumniID := uint64(104)
	before := &model.AlumniProfile{ID: alumniID, Name: "周七", Grade: "2018级", WorkUnit: &oldUnit, Status: common.AlumniStatusActive}
	after := &model.AlumniProfile{ID: alumniID, Name: "周七", Grade: "2018级", WorkUnit: &newUnit, Status: common.AlumniStatusActive}
	store := &fakeAlumniStore{detail: before, updateResult: after}
	users := &fakeUserStore{usersByID: map[uint64]*model.User{7: {ID: 7, Account: "alumni", Role: common.RoleAlumni, RealName: ptr("周七")}}}
	writer := &captureAuditWriter{}
	access := common.AccessContext{UserID: 7, Role: common.RoleAlumni, AlumniID: &alumniID}

	_, err := NewAlumniService(store, nil).
		WithAuditUserStore(users).
		WithAuditWriter(writer).
		UpdateMe(context.Background(), access, dto.AlumniProfileUpdateRequest{WorkUnit: &newUnit})
	if err != nil {
		t.Fatalf("UpdateMe() error = %v", err)
	}
	if len(writer.events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(writer.events))
	}
	event := writer.events[0]
	if event.Action != AuditActionUpdate || event.Source != "alumni_self" || len(event.Changes) != 1 || event.Changes[0].FieldName != "work_unit" {
		t.Fatalf("unexpected alumni self audit event: %+v", event)
	}
}
