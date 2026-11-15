package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/xuri/excelize/v2"
)

// fakeAlumniStore 是服务层测试使用的最小内存仓储。
// 个人档案与学籍段分离后，列表/详情返回 do.AlumniRecord；测试通过 items/detail
// 与对应的 enrollments 字段构造数据。
type fakeAlumniStore struct {
	profile   *model.AlumniProfile
	findErr   error
	updateErr error

	query           do.AlumniListQuery
	items           []*model.AlumniProfile
	itemEnrollments map[uint64][]*model.AlumniEnrollment
	total           int64
	err             error

	detailID          uint64
	getDomainIDs      []uint64
	detail            *model.AlumniProfile
	detailEnrollments []*model.AlumniEnrollment
	detailErr         error

	createInput       do.AlumniCreateInput
	createResult      *model.AlumniProfile
	createEnrollments []*model.AlumniEnrollment
	createErr         error

	saveID        uint64
	saveUserID    uint64
	saveInput     do.AlumniSaveInput
	saveDomainIDs []uint64
	saveResult    *do.AlumniSaveResult
	saveErr       error

	updateResult    *model.AlumniProfile
	updateID        uint64
	updateUserID    uint64
	updateProfile   do.AlumniEditableProfile
	updateDomainIDs []uint64

	deleteID        uint64
	deleteUserID    uint64
	deleteDomainIDs []uint64
	deleteErr       error

	batchInputs     []do.AlumniCreateInput
	batchOperatorID uint64
	batchIDs        []uint64
	batchErr        error

	existing map[string]bool

	updatedID uint64
	updatedMB string
	updatedEM string
}

func (s *fakeAlumniStore) recordsFor(profiles []*model.AlumniProfile) []do.AlumniRecord {
	records := make([]do.AlumniRecord, 0, len(profiles))
	for _, profile := range profiles {
		if profile == nil {
			continue
		}
		enrollments := s.itemEnrollments[profile.ID]
		if enrollments == nil {
			enrollments = []*model.AlumniEnrollment{}
		}
		records = append(records, do.AlumniRecord{Profile: profile, Enrollments: enrollments})
	}
	return records
}

func (s *fakeAlumniStore) List(_ context.Context, query do.AlumniListQuery) ([]do.AlumniRecord, int64, error) {
	s.query = query
	return s.recordsFor(s.items), s.total, s.err
}

func (s *fakeAlumniStore) ListAll(_ context.Context, query do.AlumniListQuery) ([]do.AlumniExportRow, error) {
	s.query = query
	rows := make([]do.AlumniExportRow, 0, len(s.items))
	for _, profile := range s.items {
		if profile == nil {
			continue
		}
		enrollments := s.itemEnrollments[profile.ID]
		for _, enrollment := range enrollments {
			rows = append(rows, do.AlumniExportRow{Profile: profile, Enrollment: enrollment})
		}
	}
	return rows, s.err
}

func (s *fakeAlumniStore) FindOnly(_ context.Context, query do.AlumniListQuery) ([]do.AlumniRecord, error) {
	s.query = query
	return s.recordsFor(s.items), s.err
}

func (s *fakeAlumniStore) CountActive(_ context.Context) (int64, error) {
	return int64(len(s.items)), s.err
}

func (s *fakeAlumniStore) GetDetail(_ context.Context, id uint64, dataDomainIDs []uint64) (*do.AlumniRecord, error) {
	s.detailID = id
	s.getDomainIDs = slices.Clone(dataDomainIDs)
	if s.detailErr != nil {
		return nil, s.detailErr
	}
	if s.detail == nil {
		if s.updateResult != nil && s.updateResult.ID == id {
			return &do.AlumniRecord{Profile: s.updateResult, Enrollments: s.detailEnrollments}, nil
		}
		return nil, common.ErrAlumniNotFound
	}
	profile := s.detail
	if s.updateID == id && s.updateResult != nil {
		profile = s.updateResult
	}
	return &do.AlumniRecord{Profile: profile, Enrollments: s.detailEnrollments}, nil
}

func (s *fakeAlumniStore) GetPersonalProfile(_ context.Context, id uint64) (*model.AlumniProfile, error) {
	s.detailID = id
	if s.detailErr != nil {
		return nil, s.detailErr
	}
	if s.updateID == id && s.updateResult != nil {
		return s.updateResult, nil
	}
	if s.detail != nil && s.detail.ID == id {
		return s.detail, nil
	}
	if s.profile != nil && s.profile.ID == id {
		return s.profile, nil
	}
	return nil, common.ErrAlumniNotFound
}

func (s *fakeAlumniStore) GetEnrollment(_ context.Context, alumniID uint64, enrollmentID uint64, dataDomainIDs []uint64) (*model.AlumniEnrollment, error) {
	s.detailID = alumniID
	s.getDomainIDs = slices.Clone(dataDomainIDs)
	for _, enrollment := range s.detailEnrollments {
		if enrollment.ID == enrollmentID && enrollment.AlumniID == alumniID {
			return enrollment, nil
		}
	}
	return nil, common.ErrEnrollmentNotFound
}

func (s *fakeAlumniStore) Create(_ context.Context, input do.AlumniCreateInput, operatorID uint64) (*do.AlumniRecord, error) {
	s.createInput = input
	s.batchOperatorID = operatorID
	if s.createErr != nil {
		return nil, s.createErr
	}
	profile := s.createResult
	if profile == nil {
		profile = &model.AlumniProfile{
			ID:        1,
			Name:      input.Personal.Name,
			Gender:    input.Personal.Gender,
			Mobile:    input.Personal.Mobile,
			Email:     input.Personal.Email,
			Industry:  input.Personal.Industry,
			WorkUnit:  input.Personal.WorkUnit,
			Position:  input.Personal.Position,
			Status:    common.AlumniStatusActive,
			Version:   1,
			CreatedBy: &operatorID,
			UpdatedBy: &operatorID,
		}
	}
	s.createEnrollments = enrollmentsFromInput(profile.ID, input.Enrollments)
	return &do.AlumniRecord{Profile: profile, Enrollments: s.createEnrollments}, nil
}

func (s *fakeAlumniStore) BatchCreate(_ context.Context, inputs []do.AlumniCreateInput, operatorID uint64) ([]uint64, error) {
	s.batchInputs = append([]do.AlumniCreateInput(nil), inputs...)
	s.batchOperatorID = operatorID
	if s.batchErr != nil {
		return nil, s.batchErr
	}
	if s.batchIDs != nil {
		return s.batchIDs, nil
	}
	ids := make([]uint64, 0, len(inputs))
	for i := range inputs {
		ids = append(ids, uint64(i+1))
	}
	return ids, nil
}

func (s *fakeAlumniStore) Save(_ context.Context, id uint64, input do.AlumniSaveInput, operatorID uint64, dataDomainIDs []uint64) (*do.AlumniSaveResult, error) {
	s.saveID = id
	s.saveUserID = operatorID
	s.saveInput = input
	s.saveDomainIDs = slices.Clone(dataDomainIDs)
	if s.saveErr != nil {
		return nil, s.saveErr
	}
	if s.saveResult != nil {
		return s.saveResult, nil
	}
	profile := s.updateResult
	if profile == nil {
		profile = s.detail
	}
	if profile == nil {
		profile = &model.AlumniProfile{ID: id, Name: input.Personal.Name, Status: common.AlumniStatusActive, Version: input.Version + 1}
	}
	enrollments := s.detailEnrollments
	if len(input.Enrollments) > 0 && len(enrollments) == 0 {
		enrollments = enrollmentsFromInput(id, input.Enrollments)
	}
	return &do.AlumniSaveResult{Visible: true, Record: &do.AlumniRecord{Profile: profile, Enrollments: enrollments}}, nil
}

func enrollmentsFromInput(alumniID uint64, inputs []do.AlumniEnrollmentInput) []*model.AlumniEnrollment {
	enrollments := make([]*model.AlumniEnrollment, 0, len(inputs))
	for i, input := range inputs {
		id := input.ID
		if id == 0 {
			id = uint64(i + 1)
		}
		enrollments = append(enrollments, &model.AlumniEnrollment{
			ID:            id,
			AlumniID:      alumniID,
			DataDomainID:  input.DataDomainID,
			TrainingLevel: input.TrainingLevel,
			Grade:         input.Grade,
			ClassName:     input.ClassName,
			Cohort:        input.Cohort,
			Counselor:     input.Counselor,
			Mentor:        input.Mentor,
			Major:         input.Major,
			TrainingMode:  input.TrainingMode,
			Remark:        input.Remark,
			Status:        common.AlumniStatusActive,
			Version:       1,
		})
	}
	return enrollments
}

func (s *fakeAlumniStore) Delete(_ context.Context, id uint64, updaterID uint64, dataDomainIDs []uint64) error {
	s.deleteID = id
	s.deleteUserID = updaterID
	s.deleteDomainIDs = slices.Clone(dataDomainIDs)
	return s.deleteErr
}

func (s *fakeAlumniStore) UpdateEditableFields(_ context.Context, id uint64, updaterID uint64, profile do.AlumniEditableProfile) error {
	s.updateID = id
	s.updateUserID = updaterID
	s.updateProfile = profile
	return s.updateErr
}

func (s *fakeAlumniStore) FindExistingByDedupKey(_ context.Context, _ []do.AlumniDedupKey) (map[string]bool, error) {
	if s.existing != nil {
		return s.existing, nil
	}
	return make(map[string]bool), nil
}

func (s *fakeAlumniStore) FindByMobile(_ context.Context, _ string) (*model.AlumniProfile, error) {
	return s.profile, s.findErr
}

func (s *fakeAlumniStore) FindByEmail(_ context.Context, _ string) (*model.AlumniProfile, error) {
	return s.profile, s.findErr
}

func (s *fakeAlumniStore) UpdateMobile(_ context.Context, id uint64, mobile string) error {
	s.updatedID = id
	s.updatedMB = mobile
	return s.updateErr
}

func (s *fakeAlumniStore) UpdateEmail(_ context.Context, id uint64, email string) error {
	s.updatedID = id
	s.updatedEM = email
	return s.updateErr
}

func buildAlumniImportWorkbook(t *testing.T, rows [][]string) *bytes.Buffer {
	t.Helper()

	f := excelize.NewFile()
	defer f.Close()
	for column, header := range alumniImportColumnHeaders {
		cell, err := excelize.CoordinatesToCellName(column+1, 1)
		if err != nil {
			t.Fatalf("header cell: %v", err)
		}
		if err := f.SetCellValue("Sheet1", cell, header); err != nil {
			t.Fatalf("set header: %v", err)
		}
	}
	for rowIndex, row := range rows {
		for column, value := range row {
			cell, err := excelize.CoordinatesToCellName(column+1, rowIndex+2)
			if err != nil {
				t.Fatalf("data cell: %v", err)
			}
			if err := f.SetCellValue("Sheet1", cell, value); err != nil {
				t.Fatalf("set data: %v", err)
			}
		}
	}

	var data bytes.Buffer
	if err := f.Write(&data); err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	return &data
}

func importRow(name, level, grade, domain string) []string {
	return []string{name, level, grade, "", "", "", "", "", "", "", "", "", "", "", "", "", domain}
}

func TestAlumniServiceImportPartiallySucceedsWhenRowContainsUnauthorizedSensitiveFields(t *testing.T) {
	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil, testDataDomainStore())
	workbook := buildAlumniImportWorkbook(t, [][]string{
		{"敏感行", "硕士", "2020级", "", "", "", "", "", "", "", "", "", "", "", "13800000000", "", "mpa"},
		{"正常行", "硕士", "2020级", "", "", "", "", "", "", "", "", "", "", "", "", "", "mpa"},
	})

	result, err := svc.Import(context.Background(), common.AccessContext{
		UserID:    7,
		Role:      common.RoleAdmin,
		DomainIDs: []uint64{2},
	}, nil, workbook)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result.Total != 2 || result.Success != 1 || len(result.Errors) != 1 {
		t.Fatalf("unexpected import result: %+v", result)
	}
	if rowError := result.Errors[0]; rowError.Row != 2 || rowError.Name != "敏感行" || rowError.Message != "无权导入敏感字段" {
		t.Fatalf("unexpected row error: %+v", rowError)
	}
	if len(store.batchInputs) != 1 || store.batchInputs[0].Personal.Name != "正常行" {
		t.Fatalf("expected only valid row to be imported, got %+v", store.batchInputs)
	}
}

func TestAlumniServiceCreateNormalizesAndMapsDetail(t *testing.T) {
	className := " 2020级MPA周末班 "
	grade := " 2020级 "
	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil)

	detail, err := svc.Create(context.Background(), common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin}, dto.AdminAlumniCreateRequest{
		Name: " 张三 ",
		Enrollments: []dto.AlumniEnrollmentPayload{{
			DataDomainID:  3,
			TrainingLevel: common.TrainingLevelMaster,
			Grade:         grade,
			ClassName:     &className,
			Mentor:        new(string),
		}},
	})
	if err != nil {
		t.Fatalf("expected create success, got %v", err)
	}
	if store.createInput.Personal.Name != "张三" {
		t.Fatalf("expected trimmed name, got %q", store.createInput.Personal.Name)
	}
	if len(store.createInput.Enrollments) != 1 {
		t.Fatalf("expected one enrollment, got %+v", store.createInput.Enrollments)
	}
	enrollment := store.createInput.Enrollments[0]
	if enrollment.Grade != "2020级" || enrollment.ClassName == nil || *enrollment.ClassName != "2020级MPA周末班" {
		t.Fatalf("unexpected normalized enrollment: %+v", enrollment)
	}
	if enrollment.Mentor != nil {
		t.Fatalf("expected blank mentor to be nil, got %+v", enrollment.Mentor)
	}
	if detail.Name != "张三" || len(detail.Enrollments) != 1 || detail.Enrollments[0].TrainingLevel != common.TrainingLevelMaster {
		t.Fatalf("unexpected created detail: %+v", detail)
	}
}

func TestAlumniServiceCreateRejectsMissingRequiredFieldsAndLevel(t *testing.T) {
	svc := NewAlumniService(&fakeAlumniStore{}, nil)

	_, err := svc.Create(context.Background(), common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin}, dto.AdminAlumniCreateRequest{
		Name: " ",
		Enrollments: []dto.AlumniEnrollmentPayload{{
			DataDomainID: 3, TrainingLevel: common.TrainingLevelMaster, Grade: "2020级",
		}},
	})
	if err != common.ErrInvalidRequest {
		t.Fatalf("expected invalid request for blank name, got %v", err)
	}

	_, err = svc.Create(context.Background(), common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin}, dto.AdminAlumniCreateRequest{
		Name: "张三",
		Enrollments: []dto.AlumniEnrollmentPayload{{
			DataDomainID: 3, TrainingLevel: common.TrainingLevelUnverified, Grade: "2020级",
		}},
	})
	if err != common.ErrInvalidTrainingLevel {
		t.Fatalf("expected unverified level to be rejected for new enrollment, got %v", err)
	}
}

func TestAlumniServiceUpdateNormalizesAndSavesAll(t *testing.T) {
	className := " 2020级MPA周末班 "
	remark := " 管理端备注 "
	updatedAt := time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC)
	store := &fakeAlumniStore{
		detail: &model.AlumniProfile{ID: 9, Name: "张三", Status: common.AlumniStatusActive, Version: 3},
		detailEnrollments: []*model.AlumniEnrollment{{
			ID: 11, AlumniID: 9, DataDomainID: 3, TrainingLevel: common.TrainingLevelMaster, Grade: "2020级",
			Status: common.AlumniStatusActive, Version: 2,
		}},
		updateResult: &model.AlumniProfile{ID: 9, Name: "张三", Status: common.AlumniStatusActive, Version: 4, UpdatedAt: updatedAt},
	}
	svc := NewAlumniService(store, nil)

	detail, err := svc.Update(context.Background(), common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin}, 9, dto.AdminAlumniUpdateRequest{
		Version: 3,
		Name:    " 张三 ",
		Enrollments: []dto.AlumniEnrollmentPayload{{
			ID:            new(uint64(11)),
			Version:       new(uint32(2)),
			DataDomainID:  3,
			TrainingLevel: common.TrainingLevelMaster,
			Grade:         "2020级",
			ClassName:     &className,
			Remark:        &remark,
		}},
	})
	if err != nil {
		t.Fatalf("expected update success, got %v", err)
	}
	if store.saveID != 9 || store.saveUserID != 7 || store.saveInput.Version != 3 {
		t.Fatalf("unexpected save target: %+v", store.saveInput)
	}
	if len(store.saveInput.Enrollments) != 1 {
		t.Fatalf("expected one saved enrollment, got %+v", store.saveInput.Enrollments)
	}
	saved := store.saveInput.Enrollments[0]
	if saved.ClassName == nil || *saved.ClassName != "2020级MPA周末班" || saved.Remark == nil || *saved.Remark != "管理端备注" {
		t.Fatalf("unexpected normalized enrollment: %+v", saved)
	}
	if detail.ID != 9 || !detail.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("unexpected updated detail: %+v", detail)
	}
}

func TestAlumniServiceUpdateRejectsVersionConflictBeforeWriting(t *testing.T) {
	store := &fakeAlumniStore{
		detail: &model.AlumniProfile{ID: 9, Name: "张三", Status: common.AlumniStatusActive, Version: 3},
		detailEnrollments: []*model.AlumniEnrollment{{
			ID: 11, AlumniID: 9, DataDomainID: 3, TrainingLevel: common.TrainingLevelMaster, Grade: "2020级",
			Status: common.AlumniStatusActive, Version: 2,
		}},
		saveErr: common.ErrVersionConflict,
	}
	svc := NewAlumniService(store, nil)

	_, err := svc.Update(context.Background(), common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin}, 9, dto.AdminAlumniUpdateRequest{
		Version: 2,
		Name:    "张三",
		Enrollments: []dto.AlumniEnrollmentPayload{{
			ID: new(uint64(11)), Version: new(uint32(2)), DataDomainID: 3,
			TrainingLevel: common.TrainingLevelMaster, Grade: "2020级",
		}},
	})
	if err != common.ErrVersionConflict {
		t.Fatalf("expected version conflict, got %v", err)
	}
}

func TestAlumniServiceUpdateRejectsHiddenEnrollmentID(t *testing.T) {
	store := &fakeAlumniStore{
		detail: &model.AlumniProfile{ID: 9, Name: "张三", Status: common.AlumniStatusActive, Version: 1},
		detailEnrollments: []*model.AlumniEnrollment{{
			ID: 11, AlumniID: 9, DataDomainID: 3, TrainingLevel: common.TrainingLevelMaster, Grade: "2020级",
			Status: common.AlumniStatusActive, Version: 1,
		}},
	}
	svc := NewAlumniService(store, nil)

	_, err := svc.Update(context.Background(), common.AccessContext{
		UserID: 7, Role: common.RoleAdmin, DomainIDs: []uint64{3},
	}, 9, dto.AdminAlumniUpdateRequest{
		Version: 1,
		Name:    "张三",
		Enrollments: []dto.AlumniEnrollmentPayload{{
			ID: new(uint64(99)), Version: new(uint32(1)), DataDomainID: 4,
			TrainingLevel: common.TrainingLevelMaster, Grade: "2020级",
		}},
	})
	if err != common.ErrEnrollmentNotFound {
		t.Fatalf("expected hidden enrollment id to be rejected, got %v", err)
	}
}

func TestAlumniServiceListNormalizesAndMapsItems(t *testing.T) {
	workUnit := "山东大学"
	position := "主任"
	gender := "女"
	email := "zhangsan@example.com"
	updatedAt := time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC)
	store := &fakeAlumniStore{
		items: []*model.AlumniProfile{{
			ID: 9, Name: "张三", Gender: &gender, Email: &email,
			WorkUnit: &workUnit, Position: &position, Version: 2, UpdatedAt: updatedAt,
		}},
		itemEnrollments: map[uint64][]*model.AlumniEnrollment{
			9: {{ID: 11, AlumniID: 9, DataDomainID: 3, TrainingLevel: common.TrainingLevelMaster, Grade: "2020级", Major: new(string)}},
		},
		total: 12,
	}
	svc := NewAlumniService(store, nil)

	pager, err := svc.List(context.Background(), dto.AlumniListRequest{
		Page:     0,
		PageSize: 1000,
		Keyword:  " 张三 ",
	}, common.AccessContext{Role: common.RoleSuperAdmin})
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
	item := pager.Items[0]
	if item.ID != 9 || item.Name != "张三" || item.WorkUnit == nil || *item.WorkUnit != workUnit {
		t.Fatalf("unexpected alumni item: %+v", item)
	}
	if len(item.Enrollments) != 1 || item.Enrollments[0].TrainingLevel != common.TrainingLevelMaster {
		t.Fatalf("expected enrollment summary to be mapped, got %+v", item.Enrollments)
	}
}

func TestAlumniServiceGetByIDMapsDetail(t *testing.T) {
	mentor := "王老师"
	mailingAddress := "济南市"
	createdAt := time.Date(2026, 4, 28, 9, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC)
	alumniID := uint64(9)
	store := &fakeAlumniStore{
		detail: &model.AlumniProfile{
			ID: alumniID, Name: "张三", MailingAddress: &mailingAddress,
			Status: "active", Version: 1, CreatedAt: createdAt, UpdatedAt: updatedAt,
		},
		detailEnrollments: []*model.AlumniEnrollment{{
			ID: 11, AlumniID: alumniID, DataDomainID: 3, TrainingLevel: common.TrainingLevelMaster,
			Grade: "2020级", Mentor: &mentor, Status: common.AlumniStatusActive, Version: 1,
		}},
	}
	svc := NewAlumniService(store, nil)

	detail, err := svc.GetByID(context.Background(), alumniID, common.AccessContext{Role: common.RoleAlumni})
	if err != nil {
		t.Fatalf("expected detail success, got %v", err)
	}
	if store.detailID != alumniID {
		t.Fatalf("expected detail id %d, got %d", alumniID, store.detailID)
	}
	if detail.ID != alumniID || detail.Name != "张三" || len(detail.Enrollments) != 1 {
		t.Fatalf("unexpected alumni detail: %+v", detail)
	}
	if detail.Enrollments[0].Mentor == nil || *detail.Enrollments[0].Mentor != mentor {
		t.Fatalf("expected mentor mapped from enrollment, got %+v", detail.Enrollments[0])
	}
	if detail.Enrollments[0].Remark != nil {
		t.Fatalf("alumni viewer must not see admin remark, got %+v", detail.Enrollments[0].Remark)
	}
	if !detail.CreatedAt.Equal(createdAt) || !detail.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("unexpected detail times: %+v", detail)
	}
}

func TestAlumniServiceGetByIDMasksSensitiveFieldsForAlumniViewingOthers(t *testing.T) {
	mobile := "13800000000"
	workUnit := "山东大学"
	position := "主任"
	mailingAddress := "济南市"
	alumniID := uint64(9)
	viewerAlumniID := uint64(5)
	store := &fakeAlumniStore{
		detail: &model.AlumniProfile{
			ID: alumniID, Name: "张三", Mobile: &mobile, WorkUnit: &workUnit,
			Position: &position, MailingAddress: &mailingAddress, Status: "active",
		},
	}
	svc := NewAlumniService(store, nil)

	detail, err := svc.GetByID(context.Background(), alumniID, common.AccessContext{Role: common.RoleAlumni, AlumniID: &viewerAlumniID})
	if err != nil {
		t.Fatalf("expected detail success, got %v", err)
	}
	if detail.Mobile != nil || detail.WorkUnit != nil || detail.Position != nil || detail.MailingAddress != nil {
		t.Fatalf("expected sensitive fields to be masked, got %+v", detail)
	}
}

func TestAlumniServiceGetByIDShowsAllFieldsForSelfAndAdmin(t *testing.T) {
	mobile := "13800000000"
	position := "主任"
	mailingAddress := "济南市"
	alumniID := uint64(9)
	newStore := func() *fakeAlumniStore {
		return &fakeAlumniStore{detail: &model.AlumniProfile{
			ID: alumniID, Name: "张三", Mobile: &mobile, Position: &position,
			MailingAddress: &mailingAddress, Status: "active",
		}}
	}

	selfDetail, err := NewAlumniService(newStore(), nil).GetByID(context.Background(), alumniID,
		common.AccessContext{Role: common.RoleAlumni, AlumniID: &alumniID})
	if err != nil {
		t.Fatalf("expected self detail success, got %v", err)
	}
	if selfDetail.Mobile == nil || *selfDetail.Mobile != mobile || selfDetail.MailingAddress == nil {
		t.Fatalf("expected self to see own sensitive fields, got %+v", selfDetail)
	}

	adminDetail, err := NewAlumniService(newStore(), nil).GetByID(context.Background(), alumniID,
		common.AccessContext{Role: common.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("expected admin detail success, got %v", err)
	}
	if adminDetail.Mobile == nil || adminDetail.Position == nil || adminDetail.MailingAddress == nil {
		t.Fatalf("expected admin to see sensitive fields, got %+v", adminDetail)
	}
}

func TestAlumniServiceGetMeUsesBoundAlumniID(t *testing.T) {
	alumniID := uint64(9)
	store := &fakeAlumniStore{detail: &model.AlumniProfile{ID: alumniID, Name: "张三", Status: "active"}}
	svc := NewAlumniService(store, nil)

	detail, err := svc.GetMe(context.Background(), common.AccessContext{UserID: 3, Role: common.RoleAlumni, AlumniID: &alumniID})
	if err != nil {
		t.Fatalf("expected me success, got %v", err)
	}
	if store.detailID != alumniID || detail.ID != alumniID {
		t.Fatalf("unexpected alumni detail: id=%d detail=%+v", store.detailID, detail)
	}
}

func TestAlumniServiceGetMeRejectsNonAlumniAndUnboundAccess(t *testing.T) {
	svc := NewAlumniService(&fakeAlumniStore{}, nil)
	if _, err := svc.GetMe(context.Background(), common.AccessContext{UserID: 3, Role: common.RoleAdmin}); err != common.ErrPermissionDenied {
		t.Fatalf("expected permission denied, got %v", err)
	}
	if _, err := svc.GetMe(context.Background(), common.AccessContext{UserID: 3, Role: common.RoleAlumni}); err != common.ErrAlumniProfileUnbound {
		t.Fatalf("expected alumni profile unbound, got %v", err)
	}
}

func TestAlumniServiceUpdateMeUpdatesOnlyEditableFields(t *testing.T) {
	alumniID := uint64(9)
	workUnit := " 山东大学 "
	position := "主任"
	mobile := " 13800000000 "
	store := &fakeAlumniStore{detail: &model.AlumniProfile{ID: alumniID, Name: "张三", Status: "active"}}
	svc := NewAlumniService(store, nil)

	_, err := svc.UpdateMe(context.Background(), common.AccessContext{UserID: 3, Role: common.RoleAlumni, AlumniID: &alumniID}, dto.AlumniProfileUpdateRequest{
		WorkUnit: &workUnit,
		Position: &position,
		Mobile:   &mobile,
	})
	if err != nil {
		t.Fatalf("expected update success, got %v", err)
	}
	if store.updateID != alumniID || store.updateUserID != 3 {
		t.Fatalf("unexpected update target: alumni=%d user=%d", store.updateID, store.updateUserID)
	}
	if store.updateProfile.WorkUnit == nil || *store.updateProfile.WorkUnit != "山东大学" {
		t.Fatalf("expected trimmed work unit, got %+v", store.updateProfile.WorkUnit)
	}
	if store.updateProfile.Position == nil || *store.updateProfile.Position != position {
		t.Fatalf("expected position update, got %+v", store.updateProfile.Position)
	}
	if store.updateProfile.MailingAddress != nil {
		t.Fatalf("expected mailing address untouched, got %+v", store.updateProfile.MailingAddress)
	}
	if store.updateProfile.Mobile == nil || *store.updateProfile.Mobile != "13800000000" {
		t.Fatalf("expected trimmed mobile, got %+v", store.updateProfile.Mobile)
	}
}

func TestAlumniServiceDeleteSuccessAndErrors(t *testing.T) {
	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil)
	if err := svc.Delete(context.Background(), common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin}, 9); err != nil {
		t.Fatalf("expected delete success, got %v", err)
	}
	if store.deleteID != 9 || store.deleteUserID != 7 {
		t.Fatalf("unexpected delete target: alumni=%d user=%d", store.deleteID, store.deleteUserID)
	}

	if err := NewAlumniService(&fakeAlumniStore{deleteErr: common.ErrAlumniNotFound}, nil).
		Delete(context.Background(), common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin}, 9); err != common.ErrAlumniNotFound {
		t.Fatalf("expected alumni not found, got %v", err)
	}
	if err := NewAlumniService(&fakeAlumniStore{deleteErr: common.ErrAlumniMultiEnrollment}, nil).
		Delete(context.Background(), common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin}, 9); err != common.ErrAlumniMultiEnrollment {
		t.Fatalf("expected multi-enrollment rejection, got %v", err)
	}
}

func TestAlumniServiceListScopesDomainAndSensitiveSearch(t *testing.T) {
	store := &fakeAlumniStore{total: 1}
	svc := NewAlumniService(store, nil)
	viewer := common.AccessContext{Role: common.RoleAdmin, DomainIDs: []uint64{2}}

	if _, err := svc.List(context.Background(), dto.AlumniListRequest{Keyword: "张三"}, viewer); err != nil {
		t.Fatalf("expected scoped list success, got %v", err)
	}
	if !slices.Equal(store.query.DataDomainIDs, []uint64{2}) {
		t.Fatalf("expected query to be limited to domain 2, got %v", store.query.DataDomainIDs)
	}
	if store.query.CanReadSensitive {
		t.Fatal("expected sensitive fields to remain unavailable")
	}

	for _, req := range []dto.AlumniListRequest{{Position: "主任"}, {WorkUnit: "山东大学"}, {Mobile: "13800000000"}} {
		if _, err := svc.List(context.Background(), req, viewer); err != common.ErrPermissionDenied {
			t.Fatalf("expected sensitive filter %+v to be rejected, got %v", req, err)
		}
	}
}

func TestAlumniServiceListHonorsRequestedDataDomain(t *testing.T) {
	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil)
	requestedDomainID := uint64(3)
	viewer := common.AccessContext{Role: common.RoleAdmin, DomainIDs: []uint64{2, 3}}

	if _, err := svc.List(context.Background(), dto.AlumniListRequest{DataDomainID: &requestedDomainID}, viewer); err != nil {
		t.Fatalf("expected requested domain list success, got %v", err)
	}
	if !slices.Equal(store.query.DataDomainIDs, []uint64{3}) {
		t.Fatalf("expected query to be limited to requested domain, got %v", store.query.DataDomainIDs)
	}

	forbiddenDomainID := uint64(4)
	if _, err := svc.List(context.Background(), dto.AlumniListRequest{DataDomainID: &forbiddenDomainID}, viewer); err != common.ErrPermissionDenied {
		t.Fatalf("expected forbidden requested domain to be rejected, got %v", err)
	}
}

func TestAlumniServiceListRejectsAdminWithoutDataDomain(t *testing.T) {
	svc := NewAlumniService(&fakeAlumniStore{}, nil)
	if _, err := svc.List(context.Background(), dto.AlumniListRequest{}, common.AccessContext{Role: common.RoleAdmin}); err != common.ErrPermissionDenied {
		t.Fatalf("expected empty data-domain scope to be rejected, got %v", err)
	}
}

func TestAlumniServiceListMasksSensitiveFieldsWithoutPermission(t *testing.T) {
	mobile := "13800000000"
	email := "zhangsan@example.com"
	workUnit := "山东大学"
	position := "主任"
	store := &fakeAlumniStore{
		items: []*model.AlumniProfile{{
			ID: 9, Name: "张三", Mobile: &mobile, Email: &email, WorkUnit: &workUnit, Position: &position,
		}},
		total: 1,
	}
	svc := NewAlumniService(store, nil)

	pager, err := svc.List(context.Background(), dto.AlumniListRequest{}, common.AccessContext{Role: common.RoleAdmin, DomainIDs: []uint64{1}})
	if err != nil {
		t.Fatalf("expected list success, got %v", err)
	}
	item := pager.Items[0]
	if item.Mobile != nil || item.Email != nil || item.WorkUnit != nil || item.Position != nil {
		t.Fatalf("expected sensitive fields to be masked, got %+v", item)
	}
}

func TestAlumniServiceDetailUpdateAndDeletePassDomainScope(t *testing.T) {
	store := &fakeAlumniStore{
		detail: &model.AlumniProfile{ID: 9, Name: "张三", Status: common.AlumniStatusActive, Version: 1},
		detailEnrollments: []*model.AlumniEnrollment{{
			ID: 11, AlumniID: 9, DataDomainID: 3, TrainingLevel: common.TrainingLevelMaster, Grade: "2020级",
			Status: common.AlumniStatusActive, Version: 1,
		}},
		updateResult: &model.AlumniProfile{ID: 9, Name: "张三", Status: common.AlumniStatusActive, Version: 2},
	}
	svc := NewAlumniService(store, nil)
	viewer := common.AccessContext{UserID: 7, Role: common.RoleAdmin, DomainIDs: []uint64{1, 3}}

	if _, err := svc.GetByID(context.Background(), 9, viewer); err != nil {
		t.Fatalf("expected detail success, got %v", err)
	}
	if !slices.Equal(store.getDomainIDs, []uint64{1, 3}) {
		t.Fatalf("expected detail domain scope, got %v", store.getDomainIDs)
	}

	if _, err := svc.Update(context.Background(), viewer, 9, dto.AdminAlumniUpdateRequest{
		Version: 1, Name: "张三",
		Enrollments: []dto.AlumniEnrollmentPayload{{
			ID: new(uint64(11)), Version: new(uint32(1)), DataDomainID: 3,
			TrainingLevel: common.TrainingLevelMaster, Grade: "2020级",
		}},
	}); err != nil {
		t.Fatalf("expected update success, got %v", err)
	}
	if !slices.Equal(store.saveDomainIDs, []uint64{1, 3}) {
		t.Fatalf("expected update domain scope, got %v", store.saveDomainIDs)
	}

	if err := svc.Delete(context.Background(), viewer, 9); err != nil {
		t.Fatalf("expected delete success, got %v", err)
	}
	if !slices.Equal(store.deleteDomainIDs, []uint64{1, 3}) {
		t.Fatalf("expected delete domain scope, got %v", store.deleteDomainIDs)
	}
}

func TestAlumniServiceCreateEnforcesOperatorDomainAndSensitiveWritePermission(t *testing.T) {
	allowedDomainID := uint64(2)
	forbiddenDomainID := uint64(1)
	workUnit := "山东大学"
	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil)

	_, err := svc.Create(context.Background(), common.AccessContext{
		UserID: 7, Role: common.RoleAdmin, DomainIDs: []uint64{allowedDomainID},
	}, dto.AdminAlumniCreateRequest{
		Name: "张三",
		Enrollments: []dto.AlumniEnrollmentPayload{{
			DataDomainID: forbiddenDomainID, TrainingLevel: common.TrainingLevelMaster, Grade: "2020级",
		}},
	})
	if err != nil {
		t.Fatalf("expected single-domain create success, got %v", err)
	}
	if got := store.createInput.Enrollments[0].DataDomainID; got != allowedDomainID {
		t.Fatalf("expected forced domain %d, got %d", allowedDomainID, got)
	}

	_, err = svc.Create(context.Background(), common.AccessContext{
		UserID: 8, Role: common.RoleAdmin, DomainIDs: []uint64{1, 2},
	}, dto.AdminAlumniCreateRequest{
		Name: "李四", WorkUnit: &workUnit,
		Enrollments: []dto.AlumniEnrollmentPayload{{
			DataDomainID: forbiddenDomainID, TrainingLevel: common.TrainingLevelMaster, Grade: "2020级",
		}},
	})
	if err != common.ErrPermissionDenied {
		t.Fatalf("expected sensitive write to be rejected, got %v", err)
	}

	_, err = svc.Create(context.Background(), common.AccessContext{
		UserID: 8, Role: common.RoleAdmin, DomainIDs: []uint64{1, 2},
	}, dto.AdminAlumniCreateRequest{
		Name: "王五",
		Enrollments: []dto.AlumniEnrollmentPayload{{
			DataDomainID: forbiddenDomainID, TrainingLevel: common.TrainingLevelMaster, Grade: "2020级",
		}},
	})
	if err != nil {
		t.Fatalf("expected multi-domain create in allowed domain, got %v", err)
	}
	if got := store.createInput.Enrollments[0].DataDomainID; got != forbiddenDomainID {
		t.Fatalf("expected selected allowed domain %d, got %d", forbiddenDomainID, got)
	}
}

func TestAlumniServiceRejectsSensitiveUpdateWithoutPermission(t *testing.T) {
	workUnit := "山东大学"
	svc := NewAlumniService(&fakeAlumniStore{}, nil)
	_, err := svc.Update(context.Background(), common.AccessContext{UserID: 7, Role: common.RoleAdmin, DomainIDs: []uint64{1}},
		9, dto.AdminAlumniUpdateRequest{Name: "张三", WorkUnit: &workUnit})
	if err != common.ErrPermissionDenied {
		t.Fatalf("expected sensitive update to be rejected, got %v", err)
	}
}

func TestAlumniServiceRejectsAlumniAsAdminOperator(t *testing.T) {
	svc := NewAlumniService(&fakeAlumniStore{}, nil)
	operator := common.AccessContext{UserID: 7, Role: common.RoleAlumni}

	if _, err := svc.Create(context.Background(), operator, dto.AdminAlumniCreateRequest{Name: "张三"}); err != common.ErrPermissionDenied {
		t.Fatalf("expected create to reject alumni operator, got %v", err)
	}
	if _, err := svc.Update(context.Background(), operator, 9, dto.AdminAlumniUpdateRequest{Name: "张三"}); err != common.ErrPermissionDenied {
		t.Fatalf("expected update to reject alumni operator, got %v", err)
	}
	if err := svc.Delete(context.Background(), operator, 9); err != common.ErrPermissionDenied {
		t.Fatalf("expected delete to reject alumni operator, got %v", err)
	}
}

func TestAlumniServiceImportKeepsRowsAsIndependentPeople(t *testing.T) {
	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil, testDataDomainStore())
	workbook := buildAlumniImportWorkbook(t, [][]string{
		{"张同学", "本科", "2015", "", "", "", "", "行政管理", "", "", "", "", "", "", "13800000001", "", "undergraduate"},
		{"张同学", "硕士", "2019", "", "", "", "", "行政管理", "", "", "", "", "", "", "13800000001", "", "academic_graduate"},
	})
	result, err := svc.Import(context.Background(), common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin}, nil, workbook)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result.Total != 2 || result.Success != 2 || len(result.Errors) != 0 {
		t.Fatalf("unexpected import result: %+v", result)
	}
	if len(store.batchInputs) != 2 || len(store.batchInputs[0].Enrollments) != 1 || len(store.batchInputs[1].Enrollments) != 1 {
		t.Fatalf("expected independent people, each with an initial enrollment, got %+v", store.batchInputs)
	}
}

func TestAlumniServiceImportRejectsDuplicateEnrollment(t *testing.T) {
	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil, testDataDomainStore())
	workbook := buildAlumniImportWorkbook(t, [][]string{
		importRow("张同学", "硕士", "2019", "mpa"),
		importRow("张同学", "硕士", "2019", "mpa"),
	})
	result, err := svc.Import(context.Background(), common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin}, nil, workbook)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result.Success != 1 || len(result.Errors) != 1 || result.Errors[0].Row != 3 {
		t.Fatalf("unexpected duplicate import result: %+v", result)
	}
	if len(store.batchInputs) != 1 || len(store.batchInputs[0].Enrollments) != 1 {
		t.Fatalf("expected only one enrollment to be imported, got %+v", store.batchInputs)
	}
}

func TestAlumniServiceSaveEnforcesFilePermission(t *testing.T) {
	for _, action := range []string{"activate", "remove", "personal"} {
		for _, role := range []string{"", common.RoleAlumni, common.RoleAdmin, common.RoleSuperAdmin} {
			for _, allowed := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/%t", action, role, allowed), func(t *testing.T) {
					store := &fakeAlumniStore{detail: &model.AlumniProfile{ID: 9, Name: "张三", Version: 1}, detailEnrollments: []*model.AlumniEnrollment{fileTestEnrollment()}}
					operator := common.AccessContext{UserID: 7, Role: role, DomainIDs: []uint64{2}, Permissions: map[string]bool{common.PermissionAlumniFilesManage: allowed}}
					enrollment := dto.AlumniEnrollmentPayload{ID: new(uint64(11)), Version: new(uint32(1)), DataDomainID: 2, TrainingLevel: common.TrainingLevelMaster, Grade: "2020级"}
					if action == "activate" {
						enrollment.ActivateFileIDs = []uint64{42}
					}
					if action == "remove" {
						enrollment.RemoveFileIDs = []uint64{42}
					}
					_, err := NewAlumniService(store, nil).Update(context.Background(), operator, 9, dto.AdminAlumniUpdateRequest{Version: 1, Name: "张三", Enrollments: []dto.AlumniEnrollmentPayload{enrollment}})
					canSave := role == common.RoleSuperAdmin || (role == common.RoleAdmin && (allowed || action == "personal"))
					if canSave {
						if err != nil || store.saveID != 9 {
							t.Fatalf("authorized save failed: %v", err)
						}
					} else if !errors.Is(err, common.ErrPermissionDenied) || store.saveID != 0 {
						t.Fatalf("unauthorized save: err=%v, wrote=%d", err, store.saveID)
					}
				})
			}
		}
	}
}
