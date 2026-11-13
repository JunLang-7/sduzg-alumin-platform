package service

import (
	"context"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
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

func auditEnrollment(id, alumniID uint64) *model.AlumniEnrollment {
	return &model.AlumniEnrollment{
		ID: id, AlumniID: alumniID, DataDomainID: 3, TrainingLevel: common.TrainingLevelMaster,
		Grade: "2020级", Status: common.AlumniStatusActive, Version: 1,
	}
}

func TestAlumniServiceCreateWritesPersonalAndEnrollmentCreateAudit(t *testing.T) {
	created := &model.AlumniProfile{ID: 101, Name: "李四", Status: common.AlumniStatusActive, Version: 1}
	store := &fakeAlumniStore{createResult: created}
	users := &fakeUserStore{usersByID: map[uint64]*model.User{7: auditOperatorUser()}}
	writer := &captureAuditWriter{}

	_, err := NewAlumniService(store, nil).
		WithAuditUserStore(users).
		WithAuditWriter(writer).
		Create(context.Background(), auditOperator(), dto.AdminAlumniCreateRequest{
			Name: created.Name,
			Enrollments: []dto.AlumniEnrollmentPayload{{
				DataDomainID: 3, TrainingLevel: common.TrainingLevelMaster, Grade: "2021级",
			}},
		})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(writer.events) != 2 {
		t.Fatalf("audit events = %d, want personal + enrollment", len(writer.events))
	}
	personal := writer.events[0]
	if personal.Action != AuditActionCreate || personal.Source != "admin" || personal.TargetID == nil || *personal.TargetID != created.ID {
		t.Fatalf("unexpected create audit event: %+v", personal)
	}
	if len(personal.Changes) == 0 || personal.Changes[0].FieldName != "name" {
		t.Fatalf("create audit changes = %+v, want name diff", personal.Changes)
	}
	enrollment := writer.events[1]
	if enrollment.EnrollmentID == nil || enrollment.TargetName != created.Name || enrollment.EnrollmentLabel == "" {
		t.Fatalf("unexpected enrollment create audit: %+v", enrollment)
	}
	if len(enrollment.DataDomainIDs) != 1 || enrollment.DataDomainIDs[0] != 3 {
		t.Fatalf("enrollment audit domains = %v, want [3]", enrollment.DataDomainIDs)
	}
}

func TestAlumniServiceUpdateWritesOnlyChangedFields(t *testing.T) {
	oldUnit := "旧单位"
	newUnit := "新单位"
	before := &model.AlumniProfile{ID: 102, Name: "王五", WorkUnit: &oldUnit, Status: common.AlumniStatusActive, Version: 2}
	after := &model.AlumniProfile{ID: 102, Name: "王五", WorkUnit: &newUnit, Status: common.AlumniStatusActive, Version: 3}
	enrollment := auditEnrollment(12, before.ID)
	store := &fakeAlumniStore{
		detail:            before,
		detailEnrollments: []*model.AlumniEnrollment{enrollment},
		updateResult:      after,
	}
	users := &fakeUserStore{usersByID: map[uint64]*model.User{7: auditOperatorUser()}}
	writer := &captureAuditWriter{}

	_, err := NewAlumniService(store, nil).
		WithAuditUserStore(users).
		WithAuditWriter(writer).
		Update(context.Background(), auditOperator(), before.ID, dto.AdminAlumniUpdateRequest{
			Version:  2,
			Name:     before.Name,
			WorkUnit: &newUnit,
			Enrollments: []dto.AlumniEnrollmentPayload{{
				ID: new(uint64(12)), Version: new(uint32(1)), DataDomainID: 3,
				TrainingLevel: common.TrainingLevelMaster, Grade: "2020级",
			}},
		})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if len(writer.events) != 1 {
		t.Fatalf("audit events = %d, want 1 personal update", len(writer.events))
	}
	event := writer.events[0]
	if event.Action != AuditActionUpdate || len(event.Changes) != 1 || event.Changes[0].FieldName != "work_unit" {
		t.Fatalf("unexpected update audit event: %+v", event)
	}
}

func TestAlumniServiceUpdateWritesEnrollmentAuditForChangedSegment(t *testing.T) {
	before := &model.AlumniProfile{ID: 105, Name: "钱八", Status: common.AlumniStatusActive, Version: 1}
	after := &model.AlumniProfile{ID: 105, Name: "钱八", Status: common.AlumniStatusActive, Version: 2}
	enrollment := auditEnrollment(15, before.ID)
	updated := *enrollment
	updated.Major = new("公共管理")
	updated.Version = 2
	store := &fakeAlumniStore{
		detail:            before,
		detailEnrollments: []*model.AlumniEnrollment{enrollment},
		updateResult:      after,
	}
	// 保存后的学籍由 Save 返回，这里让 fake 返回更新后的学籍。
	store.saveResult = &do.AlumniSaveResult{Record: do.AlumniRecord{Profile: after, Enrollments: []*model.AlumniEnrollment{&updated}}}
	users := &fakeUserStore{usersByID: map[uint64]*model.User{7: auditOperatorUser()}}
	writer := &captureAuditWriter{}

	_, err := NewAlumniService(store, nil).
		WithAuditUserStore(users).
		WithAuditWriter(writer).
		Update(context.Background(), auditOperator(), before.ID, dto.AdminAlumniUpdateRequest{
			Version: 1,
			Name:    before.Name,
			Enrollments: []dto.AlumniEnrollmentPayload{{
				ID: new(uint64(15)), Version: new(uint32(1)), DataDomainID: 3,
				TrainingLevel: common.TrainingLevelMaster, Grade: "2020级", Major: new("公共管理"),
			}},
		})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if len(writer.events) != 1 {
		t.Fatalf("audit events = %d, want 1 enrollment update", len(writer.events))
	}
	event := writer.events[0]
	if event.EnrollmentID == nil || *event.EnrollmentID != 15 {
		t.Fatalf("expected enrollment id 15 in audit, got %+v", event.EnrollmentID)
	}
	if len(event.Changes) != 1 || event.Changes[0].FieldName != "major" {
		t.Fatalf("unexpected enrollment audit changes: %+v", event.Changes)
	}
}

func TestAlumniServiceDeleteWritesDeleteAudit(t *testing.T) {
	before := &model.AlumniProfile{ID: 103, Name: "赵六", Status: common.AlumniStatusActive, Version: 1}
	store := &fakeAlumniStore{detail: before, detailEnrollments: []*model.AlumniEnrollment{auditEnrollment(13, before.ID)}}
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
	before := &model.AlumniProfile{ID: alumniID, Name: "周七", WorkUnit: &oldUnit, Status: common.AlumniStatusActive, Version: 1}
	after := &model.AlumniProfile{ID: alumniID, Name: "周七", WorkUnit: &newUnit, Status: common.AlumniStatusActive, Version: 2}
	store := &fakeAlumniStore{
		detail:            before,
		detailEnrollments: []*model.AlumniEnrollment{auditEnrollment(14, alumniID)},
		updateResult:      after,
	}
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
	if event.Source != "alumni_self" || event.Operator == nil || event.Operator.Role != common.RoleAlumni {
		t.Fatalf("unexpected alumni self audit event: %+v", event)
	}
	if len(event.Changes) != 1 || event.Changes[0].FieldName != "work_unit" {
		t.Fatalf("unexpected work-unit change: %+v", event.Changes)
	}
}
