package service

import (
	"context"
	"errors"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
)

func lifecycleProfile() *model.AlumniProfile {
	return &model.AlumniProfile{ID: 42, Name: "李四", Status: common.AlumniStatusActive, Version: 3}
}

func lifecycleEnrollment(id, domainID uint64, level, grade string) *model.AlumniEnrollment {
	return &model.AlumniEnrollment{
		ID: id, AlumniID: 42, DataDomainID: domainID, TrainingLevel: level, Grade: grade,
		Status: common.AlumniStatusActive, Version: 1,
	}
}

func lifecyclePayload(id uint64, version uint32, domainID uint64, level, grade string) dto.AlumniEnrollmentPayload {
	return dto.AlumniEnrollmentPayload{
		ID: new(id), Version: new(version), DataDomainID: domainID,
		TrainingLevel: level, Grade: grade,
	}
}

func lifecycleService(store *fakeAlumniStore) *AlumniService {
	return NewAlumniService(store, nil, testDataDomainStore())
}

func TestAlumniServiceRejectsIncompatibleTrainingLevelAndCategory(t *testing.T) {
	cases := []struct {
		name        string
		level       string
		domainID    uint64
		version     uint32
		wantErr     error
		expectSaved bool
	}{
		{name: "博士+MPA", level: common.TrainingLevelDoctor, domainID: 2, version: 1, wantErr: common.ErrEnrollmentCategoryMismatch},
		{name: "本科+学术学位", level: common.TrainingLevelUndergraduate, domainID: 3, version: 1, wantErr: common.ErrEnrollmentCategoryMismatch},
		{name: "硕士+学术学位", level: common.TrainingLevelMaster, domainID: 3, version: 1, expectSaved: true},
		{name: "博士+学术学位", level: common.TrainingLevelDoctor, domainID: 3, version: 1, expectSaved: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			store := &fakeAlumniStore{
				detail:            lifecycleProfile(),
				detailEnrollments: []*model.AlumniEnrollment{lifecycleEnrollment(11, 2, common.TrainingLevelMaster, "2020级")},
			}
			operator := common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin}
			_, err := lifecycleService(store).Update(context.Background(), operator, 42, dto.AdminAlumniUpdateRequest{
				Version:     3,
				Name:        "李四",
				Enrollments: []dto.AlumniEnrollmentPayload{lifecyclePayload(11, testCase.version, testCase.domainID, testCase.level, "2020级")},
			})
			if testCase.expectSaved {
				if err != nil {
					t.Fatalf("expected %s to save, got %v", testCase.name, err)
				}
				return
			}
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("expected %v, got %v", testCase.wantErr, err)
			}
			if store.saveID != 0 {
				t.Fatalf("invalid combination must not reach the store, saved %d", store.saveID)
			}
		})
	}
}

func TestAlumniServiceUnverifiedLegacyEnrollmentKeepsCategory(t *testing.T) {
	store := &fakeAlumniStore{
		detail:            lifecycleProfile(),
		detailEnrollments: []*model.AlumniEnrollment{lifecycleEnrollment(11, 3, common.TrainingLevelUnverified, "2012级")},
	}
	operator := common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin}
	if _, err := lifecycleService(store).Update(context.Background(), operator, 42, dto.AdminAlumniUpdateRequest{
		Version:     3,
		Name:        "李四",
		Enrollments: []dto.AlumniEnrollmentPayload{lifecyclePayload(11, 1, 3, common.TrainingLevelUnverified, "2012级")},
	}); err != nil {
		t.Fatalf("legacy unverified enrollment should stay editable, got %v", err)
	}
}

func TestAlumniServiceCategoryCorrectionRequiresTargetDomainPermission(t *testing.T) {
	cases := []struct {
		name      string
		role      string
		domainIDs []uint64
		target    uint64
		wantErr   error
	}{
		{name: "单域管理员跨域", role: common.RoleAdmin, domainIDs: []uint64{2}, target: 3, wantErr: common.ErrPermissionDenied},
		{name: "双域管理员更正", role: common.RoleAdmin, domainIDs: []uint64{2, 3}, target: 3},
		{name: "超级管理员更正", role: common.RoleSuperAdmin, target: 3},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			store := &fakeAlumniStore{
				detail:            lifecycleProfile(),
				detailEnrollments: []*model.AlumniEnrollment{lifecycleEnrollment(11, 2, common.TrainingLevelMaster, "2020级")},
			}
			operator := common.AccessContext{UserID: 7, Role: testCase.role, DomainIDs: testCase.domainIDs}
			_, err := lifecycleService(store).Update(context.Background(), operator, 42, dto.AdminAlumniUpdateRequest{
				Version:     3,
				Name:        "李四",
				Enrollments: []dto.AlumniEnrollmentPayload{lifecyclePayload(11, 1, testCase.target, common.TrainingLevelMaster, "2020级")},
			})
			if testCase.wantErr != nil {
				if !errors.Is(err, testCase.wantErr) {
					t.Fatalf("expected %v, got %v", testCase.wantErr, err)
				}
				if store.saveID != 0 {
					t.Fatalf("unauthorized correction reached the store")
				}
				return
			}
			if err != nil {
				t.Fatalf("correction should succeed, got %v", err)
			}
			if len(store.saveInput.Enrollments) != 1 || store.saveInput.Enrollments[0].DataDomainID != testCase.target {
				t.Fatalf("target domain not forwarded: %+v", store.saveInput.Enrollments)
			}
		})
	}
}

func TestAlumniServiceRejectsDeletingHiddenEnrollment(t *testing.T) {
	store := &fakeAlumniStore{
		detail:            lifecycleProfile(),
		detailEnrollments: []*model.AlumniEnrollment{lifecycleEnrollment(11, 2, common.TrainingLevelMaster, "2020级")},
	}
	operator := common.AccessContext{UserID: 7, Role: common.RoleAdmin, DomainIDs: []uint64{2}}
	payload := dto.AlumniEnrollmentPayload{ID: new(uint64(99)), Version: new(uint32(1)), Deleted: true}
	_, err := lifecycleService(store).Update(context.Background(), operator, 42, dto.AdminAlumniUpdateRequest{
		Version:     3,
		Name:        "李四",
		Enrollments: []dto.AlumniEnrollmentPayload{payload},
	})
	if !errors.Is(err, common.ErrEnrollmentNotFound) {
		t.Fatalf("expected ErrEnrollmentNotFound for hidden enrollment, got %v", err)
	}
	if store.saveID != 0 {
		t.Fatalf("hidden enrollment deletion reached the store")
	}
}

func TestAlumniServiceDeleteEnrollmentRequiresVersionAndNoFileChanges(t *testing.T) {
	cases := []struct {
		name    string
		payload dto.AlumniEnrollmentPayload
		wantErr error
	}{
		{
			name:    "缺少版本",
			payload: dto.AlumniEnrollmentPayload{ID: new(uint64(11)), Deleted: true},
			wantErr: common.ErrInvalidRequest,
		},
		{
			name: "删除时提交文件变更",
			payload: dto.AlumniEnrollmentPayload{
				ID: new(uint64(11)), Version: new(uint32(1)), Deleted: true, RemoveFileIDs: []uint64{42},
			},
			wantErr: common.ErrInvalidRequest,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			store := &fakeAlumniStore{
				detail:            lifecycleProfile(),
				detailEnrollments: []*model.AlumniEnrollment{lifecycleEnrollment(11, 2, common.TrainingLevelMaster, "2020级")},
			}
			operator := common.AccessContext{UserID: 7, Role: common.RoleAdmin, DomainIDs: []uint64{2}, Permissions: map[string]bool{common.PermissionAlumniFilesManage: true}}
			_, err := lifecycleService(store).Update(context.Background(), operator, 42, dto.AdminAlumniUpdateRequest{
				Version:     3,
				Name:        "李四",
				Enrollments: []dto.AlumniEnrollmentPayload{testCase.payload},
			})
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("expected %v, got %v", testCase.wantErr, err)
			}
			if store.saveID != 0 {
				t.Fatalf("invalid delete request reached the store")
			}
		})
	}
}

func TestAlumniServiceDeleteEnrollmentForwardsExplicitFlag(t *testing.T) {
	store := &fakeAlumniStore{
		detail:            lifecycleProfile(),
		detailEnrollments: []*model.AlumniEnrollment{lifecycleEnrollment(11, 2, common.TrainingLevelMaster, "2020级")},
	}
	operator := common.AccessContext{UserID: 7, Role: common.RoleAdmin, DomainIDs: []uint64{2}}
	if _, err := lifecycleService(store).Update(context.Background(), operator, 42, dto.AdminAlumniUpdateRequest{
		Version: 3,
		Name:    "李四",
		Enrollments: []dto.AlumniEnrollmentPayload{
			{ID: new(uint64(11)), Version: new(uint32(1)), Deleted: true},
		},
	}); err != nil {
		t.Fatalf("explicit delete should be forwarded, got %v", err)
	}
	if len(store.saveInput.Enrollments) != 1 || !store.saveInput.Enrollments[0].Deleted || store.saveInput.Enrollments[0].ID != 11 {
		t.Fatalf("delete flag not forwarded: %+v", store.saveInput.Enrollments)
	}
}

func TestAlumniServiceLastEnrollmentConflictIsReturned(t *testing.T) {
	store := &fakeAlumniStore{
		detail:            lifecycleProfile(),
		detailEnrollments: []*model.AlumniEnrollment{lifecycleEnrollment(11, 2, common.TrainingLevelMaster, "2020级")},
		saveErr:           common.ErrLastEnrollment,
	}
	operator := common.AccessContext{UserID: 7, Role: common.RoleAdmin, DomainIDs: []uint64{2}}
	_, err := lifecycleService(store).Update(context.Background(), operator, 42, dto.AdminAlumniUpdateRequest{
		Version: 3,
		Name:    "李四",
		Enrollments: []dto.AlumniEnrollmentPayload{
			{ID: new(uint64(11)), Version: new(uint32(1)), Deleted: true},
		},
	})
	if !errors.Is(err, common.ErrLastEnrollment) {
		t.Fatalf("expected ErrLastEnrollment, got %v", err)
	}
}

func TestAlumniServiceHiddenAfterSaveReturnsSuccessWithoutDetail(t *testing.T) {
	profile := lifecycleProfile()
	profile.Version = 4
	store := &fakeAlumniStore{
		detail:            profile,
		detailEnrollments: []*model.AlumniEnrollment{lifecycleEnrollment(11, 2, common.TrainingLevelMaster, "2020级")},
		saveResult: &do.AlumniSaveResult{
			Visible: false,
			Record:  &do.AlumniRecord{Profile: profile, Enrollments: []*model.AlumniEnrollment{}},
		},
	}
	operator := common.AccessContext{UserID: 7, Role: common.RoleAdmin, DomainIDs: []uint64{2}}
	detail, err := lifecycleService(store).Update(context.Background(), operator, 42, dto.AdminAlumniUpdateRequest{
		Version: 3,
		Name:    "李四",
		Enrollments: []dto.AlumniEnrollmentPayload{
			{ID: new(uint64(11)), Version: new(uint32(1)), Deleted: true},
		},
	})
	if err != nil {
		t.Fatalf("save that hides the alumni should still succeed, got %v", err)
	}
	if detail != nil {
		t.Fatalf("expected nil detail when the operator lost access, got %+v", detail)
	}
}

func TestAlumniServiceDeleteEnrollmentWritesScopedAudit(t *testing.T) {
	profile := lifecycleProfile()
	enrollment := lifecycleEnrollment(11, 2, common.TrainingLevelMaster, "2020级")
	store := &fakeAlumniStore{
		detail:            profile,
		detailEnrollments: []*model.AlumniEnrollment{enrollment},
		saveResult: &do.AlumniSaveResult{
			Visible: true,
			Record:  &do.AlumniRecord{Profile: profile, Enrollments: []*model.AlumniEnrollment{}},
		},
	}
	writer := &captureAuditWriter{}
	svc := lifecycleService(store).
		WithAuditUserStore(&fakeUserStore{usersByID: map[uint64]*model.User{7: auditOperatorUser()}}).
		WithAuditWriter(writer)
	operator := common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin}
	if _, err := svc.Update(context.Background(), operator, 42, dto.AdminAlumniUpdateRequest{
		Version: 3,
		Name:    "李四",
		Enrollments: []dto.AlumniEnrollmentPayload{
			{ID: new(uint64(11)), Version: new(uint32(1)), Deleted: true},
		},
	}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if len(writer.events) != 1 {
		t.Fatalf("audit events = %d, want 1 delete event", len(writer.events))
	}
	event := writer.events[0]
	if event.Action != AuditActionDelete || event.EnrollmentID == nil || *event.EnrollmentID != 11 {
		t.Fatalf("unexpected delete audit: %+v", event)
	}
	if len(event.Changes) != 1 || event.Changes[0].FieldName != "status" {
		t.Fatalf("unexpected delete changes: %+v", event.Changes)
	}
	if len(event.DataDomainIDs) != 1 || event.DataDomainIDs[0] != 2 {
		t.Fatalf("delete audit scope = %v, want the enrollment domain", event.DataDomainIDs)
	}
}

func TestAlumniServiceCategoryCorrectionAuditUsesTargetDomainOnly(t *testing.T) {
	profile := lifecycleProfile()
	before := lifecycleEnrollment(11, 2, common.TrainingLevelMaster, "2020级")
	after := *before
	after.DataDomainID = 3
	withDomain := "公共管理"
	after.Major = &withDomain
	store := &fakeAlumniStore{
		detail:            profile,
		detailEnrollments: []*model.AlumniEnrollment{before},
		saveResult: &do.AlumniSaveResult{
			Visible: true,
			Record:  &do.AlumniRecord{Profile: profile, Enrollments: []*model.AlumniEnrollment{&after}},
		},
	}
	writer := &captureAuditWriter{}
	svc := lifecycleService(store).
		WithAuditUserStore(&fakeUserStore{usersByID: map[uint64]*model.User{7: auditOperatorUser()}}).
		WithAuditWriter(writer)
	operator := common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin}
	if _, err := svc.Update(context.Background(), operator, 42, dto.AdminAlumniUpdateRequest{
		Version: 3,
		Name:    "李四",
		Enrollments: []dto.AlumniEnrollmentPayload{
			{ID: new(uint64(11)), Version: new(uint32(1)), DataDomainID: 3, TrainingLevel: common.TrainingLevelMaster, Grade: "2020级"},
		},
	}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if len(writer.events) != 1 {
		t.Fatalf("audit events = %d, want 1 correction event", len(writer.events))
	}
	event := writer.events[0]
	if len(event.DataDomainIDs) != 1 || event.DataDomainIDs[0] != 3 {
		t.Fatalf("cross-domain correction audit scope = %v, want only the target domain", event.DataDomainIDs)
	}
	var domainChange *AuditChange
	for index := range event.Changes {
		if event.Changes[index].FieldName == "data_domain_id" {
			domainChange = &event.Changes[index]
		}
	}
	if domainChange == nil {
		t.Fatalf("missing 培养类别 change: %+v", event.Changes)
	}
	if domainChange.OldValue != "MPA专业学位研究生" || domainChange.NewValue != "学术学位研究生" {
		t.Fatalf("unexpected category diff: %+v", *domainChange)
	}
}
