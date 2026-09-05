package service

import (
	"bytes"
	"context"
	"slices"
	"testing"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/xuri/excelize/v2"
)

type fakeAlumniStore struct {
	profile         *model.AlumniProfile
	findErr         error
	query           do.AlumniListQuery
	items           []*model.AlumniProfile
	total           int64
	err             error
	detailID        uint64
	getDomainIDs    []uint64
	detail          *model.AlumniProfile
	detailErr       error
	createProfile   *model.AlumniProfile
	createResult    *model.AlumniProfile
	createErr       error
	updateResult    *model.AlumniProfile
	updateID        uint64
	updateUserID    uint64
	updateProfile   do.AlumniEditableProfile
	adminUpdate     do.AlumniUpdateProfile
	updateDomainIDs []uint64
	updateErr       error
	deleteID        uint64
	deleteUserID    uint64
	deleteDomainIDs []uint64
	deleteErr       error
	batchProfiles   []do.AlumniCreateProfile
	batchOperatorID uint64
	batchErr        error
}

func (s *fakeAlumniStore) List(_ context.Context, query do.AlumniListQuery) ([]*model.AlumniProfile, int64, error) {
	s.query = query
	return s.items, s.total, s.err
}

func (s *fakeAlumniStore) ListAll(_ context.Context, query do.AlumniListQuery) ([]*model.AlumniProfile, error) {
	s.query = query
	return s.items, s.err
}

func (s *fakeAlumniStore) GetByID(_ context.Context, id uint64, dataDomainIDs []uint64) (*model.AlumniProfile, error) {
	s.detailID = id
	s.getDomainIDs = slices.Clone(dataDomainIDs)
	return s.detail, s.detailErr
}

func (s *fakeAlumniStore) Create(_ context.Context, profile *do.AlumniCreateProfile, operatorID uint64) (*model.AlumniProfile, error) {
	dataDomainID := uint64(0)
	if profile.DataDomainID != nil {
		dataDomainID = *profile.DataDomainID
	}
	s.createProfile = &model.AlumniProfile{
		DataDomainID:   dataDomainID,
		Name:           profile.Name,
		Grade:          profile.Grade,
		ClassName:      profile.ClassName,
		Cohort:         profile.Cohort,
		Counselor:      profile.Counselor,
		Mentor:         profile.Mentor,
		Major:          profile.Major,
		TrainingMode:   profile.TrainingMode,
		Industry:       profile.Industry,
		WorkUnit:       profile.WorkUnit,
		Position:       profile.Position,
		MailingAddress: profile.MailingAddress,
		Gender:         profile.Gender,
		Mobile:         profile.Mobile,
		Remark:         profile.Remark,
		Status:         profile.Status,
		CreatedBy:      &operatorID,
		UpdatedBy:      &operatorID,
	}
	if s.createResult != nil || s.createErr != nil {
		return s.createResult, s.createErr
	}
	return s.createProfile, nil
}

func (s *fakeAlumniStore) Update(_ context.Context, id uint64, updaterID uint64, profile do.AlumniUpdateProfile, dataDomainIDs []uint64) error {
	s.updateID = id
	s.updateUserID = updaterID
	s.adminUpdate = profile
	s.updateDomainIDs = slices.Clone(dataDomainIDs)
	if s.updateResult != nil {
		s.detail = s.updateResult
	}
	return s.updateErr
}

func (s *fakeAlumniStore) UpdateEditableFields(_ context.Context, id uint64, updaterID uint64, profile do.AlumniEditableProfile) error {
	s.updateID = id
	s.updateUserID = updaterID
	s.updateProfile = profile
	return s.updateErr
}

func (s *fakeAlumniStore) BatchCreate(_ context.Context, profiles []do.AlumniCreateProfile, operatorID uint64) error {
	s.batchProfiles = slices.Clone(profiles)
	s.batchOperatorID = operatorID
	return s.batchErr
}

func (s *fakeAlumniStore) CountActive(_ context.Context) (int64, error) {
	return int64(len(s.items)), s.err
}

func (s *fakeAlumniStore) FindOnly(_ context.Context, query do.AlumniListQuery) ([]*model.AlumniProfile, error) {
	s.query = query
	return s.items, s.err
}

func (s *fakeAlumniStore) FindExistingByDedupKey(_ context.Context, _ []do.AlumniDedupKey) (map[string]bool, error) {
	return make(map[string]bool), nil
}

func (s *fakeAlumniStore) Delete(_ context.Context, id uint64, updaterID uint64, dataDomainIDs []uint64) error {
	s.deleteID = id
	s.deleteUserID = updaterID
	s.deleteDomainIDs = slices.Clone(dataDomainIDs)
	return s.deleteErr
}

func buildAlumniImportWorkbook(t *testing.T, rows [][]string) *bytes.Buffer {
	t.Helper()

	f := excelize.NewFile()
	defer f.Close()
	for column, header := range alumniColumnHeaders {
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

func TestAlumniServiceImportPartiallySucceedsWhenRowContainsUnauthorizedSensitiveFields(t *testing.T) {
	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil)
	workbook := buildAlumniImportWorkbook(t, [][]string{
		{"敏感行", "2020级", "", "", "", "", "", "", "", "", "", "", "", "13800000000"},
		{"正常行", "2020级"},
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
	if len(store.batchProfiles) != 1 || store.batchProfiles[0].Name != "正常行" {
		t.Fatalf("expected only valid row to be imported, got %+v", store.batchProfiles)
	}
}

func TestAlumniServiceCreateNormalizesAndMapsDetail(t *testing.T) {
	className := " 2020级MPA周末班 "
	emptyMentor := " "
	workUnit := " 山东大学 "
	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil)

	detail, err := svc.Create(context.Background(), common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin}, dto.AdminAlumniCreateRequest{
		Name:      " 张三 ",
		Grade:     " 2020级 ",
		ClassName: &className,
		Mentor:    &emptyMentor,
		WorkUnit:  &workUnit,
	})
	if err != nil {
		t.Fatalf("expected create success, got %v", err)
	}
	if store.createProfile == nil {
		t.Fatal("expected create profile to be recorded")
	}
	if store.createProfile.Name != "张三" || store.createProfile.Grade != "2020级" {
		t.Fatalf("expected trimmed required fields, got %+v", store.createProfile)
	}
	if store.createProfile.ClassName == nil || *store.createProfile.ClassName != "2020级MPA周末班" {
		t.Fatalf("expected trimmed class name, got %+v", store.createProfile.ClassName)
	}
	if store.createProfile.Mentor != nil {
		t.Fatalf("expected blank mentor to be nil, got %+v", store.createProfile.Mentor)
	}
	if store.createProfile.WorkUnit == nil || *store.createProfile.WorkUnit != "山东大学" {
		t.Fatalf("expected trimmed work unit, got %+v", store.createProfile.WorkUnit)
	}
	if store.createProfile.CreatedBy == nil || *store.createProfile.CreatedBy != 7 {
		t.Fatalf("expected creator id 7, got %+v", store.createProfile.CreatedBy)
	}
	if store.createProfile.UpdatedBy == nil || *store.createProfile.UpdatedBy != 7 {
		t.Fatalf("expected updater id 7, got %+v", store.createProfile.UpdatedBy)
	}
	if store.createProfile.Status != common.AlumniStatusActive {
		t.Fatalf("expected active status, got %q", store.createProfile.Status)
	}
	if detail.Name != "张三" || detail.Grade != "2020级" || detail.WorkUnit == nil || *detail.WorkUnit != "山东大学" {
		t.Fatalf("unexpected created detail: %+v", detail)
	}
}

func TestAlumniServiceCreateRejectsMissingRequiredFields(t *testing.T) {
	svc := NewAlumniService(&fakeAlumniStore{}, nil)

	_, err := svc.Create(context.Background(), common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin}, dto.AdminAlumniCreateRequest{
		Name:  " ",
		Grade: "2020级",
	})
	if err != common.ErrInvalidRequest {
		t.Fatalf("expected invalid request, got %v", err)
	}
}

func TestAlumniServiceUpdateNormalizesAndMapsDetail(t *testing.T) {
	className := " 2020级MPA周末班 "
	normalizedClassName := "2020级MPA周末班"
	remark := " 管理端备注 "
	normalizedRemark := "管理端备注"
	updatedAt := time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC)
	store := &fakeAlumniStore{
		updateResult: &model.AlumniProfile{
			ID:        9,
			Name:      "张三",
			Grade:     "2020级",
			ClassName: &normalizedClassName,
			Remark:    &normalizedRemark,
			Status:    "active",
			UpdatedAt: updatedAt,
		},
	}
	svc := NewAlumniService(store, nil)

	detail, err := svc.Update(context.Background(), common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin}, 9, dto.AdminAlumniUpdateRequest{
		Name:      " 张三 ",
		Grade:     " 2020级 ",
		ClassName: &className,
		Remark:    &remark,
	})
	if err != nil {
		t.Fatalf("expected update success, got %v", err)
	}
	if store.adminUpdate.Name != "张三" || store.adminUpdate.Grade != "2020级" {
		t.Fatalf("expected trimmed required fields, got %+v", store.adminUpdate)
	}
	if store.adminUpdate.ClassName == nil || *store.adminUpdate.ClassName != "2020级MPA周末班" {
		t.Fatalf("expected trimmed class name, got %+v", store.adminUpdate.ClassName)
	}
	if store.adminUpdate.Remark == nil || *store.adminUpdate.Remark != "管理端备注" {
		t.Fatalf("expected trimmed remark, got %+v", store.adminUpdate.Remark)
	}
	if store.updateID != 9 || store.updateUserID != 7 {
		t.Fatalf("unexpected update target: alumni=%d user=%d", store.updateID, store.updateUserID)
	}
	if detail.ID != 9 || detail.Name != "张三" || detail.ClassName == nil || *detail.ClassName != "2020级MPA周末班" {
		t.Fatalf("unexpected updated detail: %+v", detail)
	}
	if !detail.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("unexpected updated time: %+v", detail.UpdatedAt)
	}
}

func TestAlumniServiceUpdatePreservesEmptyOptionalFields(t *testing.T) {
	className := " "
	remark := ""
	store := &fakeAlumniStore{
		updateResult: &model.AlumniProfile{
			ID:     9,
			Name:   "张三",
			Grade:  "2020级",
			Status: common.AlumniStatusActive,
		},
	}
	svc := NewAlumniService(store, nil)

	_, err := svc.Update(context.Background(), common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin}, 9, dto.AdminAlumniUpdateRequest{
		Name:      "张三",
		Grade:     "2020级",
		ClassName: &className,
		Remark:    &remark,
	})
	if err != nil {
		t.Fatalf("expected update success, got %v", err)
	}
	if store.adminUpdate.ClassName == nil || *store.adminUpdate.ClassName != "" {
		t.Fatalf("expected blank class name to be preserved, got %+v", store.adminUpdate.ClassName)
	}
	if store.adminUpdate.Remark == nil || *store.adminUpdate.Remark != "" {
		t.Fatalf("expected blank remark to be preserved, got %+v", store.adminUpdate.Remark)
	}
}

func TestAlumniServiceListNormalizesAndMapsItems(t *testing.T) {
	workUnit := "山东大学"
	position := "主任"
	mentor := "王老师"
	gender := "女"
	updatedAt := time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC)
	store := &fakeAlumniStore{
		items: []*model.AlumniProfile{
			{
				ID:        9,
				Name:      "张三",
				Grade:     "2020级",
				Mentor:    &mentor,
				WorkUnit:  &workUnit,
				Position:  &position,
				Gender:    &gender,
				UpdatedAt: updatedAt,
			},
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
	if pager.Items[0].ID != 9 || pager.Items[0].Name != "张三" || pager.Items[0].WorkUnit == nil || *pager.Items[0].WorkUnit != workUnit {
		t.Fatalf("unexpected alumni item: %+v", pager.Items[0])
	}
	if pager.Items[0].Mentor == nil || *pager.Items[0].Mentor != mentor || pager.Items[0].Gender == nil || *pager.Items[0].Gender != gender {
		t.Fatalf("expected mentor and gender to be mapped, got %+v", pager.Items[0])
	}
}

func TestAlumniServiceGetByIDMapsDetail(t *testing.T) {
	counselor := "李老师"
	mentor := "王老师"
	mailingAddress := "济南市"
	createdAt := time.Date(2026, 4, 28, 9, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC)
	alumniID := uint64(9)
	store := &fakeAlumniStore{
		detail: &model.AlumniProfile{
			ID:             alumniID,
			Name:           "张三",
			Grade:          "2020级",
			Counselor:      &counselor,
			Mentor:         &mentor,
			MailingAddress: &mailingAddress,
			Status:         "active",
			CreatedAt:      createdAt,
			UpdatedAt:      updatedAt,
		},
	}
	svc := NewAlumniService(store, nil)

	detail, err := svc.GetByID(context.Background(), alumniID, common.AccessContext{Role: common.RoleAlumni})
	if err != nil {
		t.Fatalf("expected detail success, got %v", err)
	}
	if store.detailID != alumniID {
		t.Fatalf("expected detail id %d, got %d", alumniID, store.detailID)
	}
	if detail.ID != alumniID || detail.Name != "张三" || detail.Counselor == nil || *detail.Counselor != counselor {
		t.Fatalf("unexpected alumni detail: %+v", detail)
	}
	if !detail.CreatedAt.Equal(createdAt) || !detail.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("unexpected detail times: %+v", detail)
	}
}

func TestAlumniServiceGetByIDMasksSensitiveFieldsForAlumniViewingOthers(t *testing.T) {
	mobile := "13800000000"
	position := "主任"
	mailingAddress := "济南市"
	alumniID := uint64(9)
	viewerAlumniID := uint64(5)
	store := &fakeAlumniStore{
		detail: &model.AlumniProfile{
			ID:             alumniID,
			Name:           "张三",
			Grade:          "2020级",
			Mobile:         &mobile,
			Position:       &position,
			MailingAddress: &mailingAddress,
			Status:         "active",
		},
	}
	svc := NewAlumniService(store, nil)

	detail, err := svc.GetByID(context.Background(), alumniID, common.AccessContext{Role: common.RoleAlumni, AlumniID: &viewerAlumniID})
	if err != nil {
		t.Fatalf("expected detail success, got %v", err)
	}
	if detail.Mobile != nil {
		t.Fatalf("expected mobile to be nil, got %v", *detail.Mobile)
	}
	if detail.Position != nil {
		t.Fatalf("expected position to be nil, got %v", *detail.Position)
	}
	if detail.MailingAddress != nil {
		t.Fatalf("expected mailing_address to be nil, got %v", *detail.MailingAddress)
	}
}

func TestAlumniServiceGetByIDShowsAllFieldsForAlumniViewingSelf(t *testing.T) {
	mobile := "13800000000"
	position := "主任"
	mailingAddress := "济南市"
	alumniID := uint64(9)
	store := &fakeAlumniStore{
		detail: &model.AlumniProfile{
			ID:             alumniID,
			Name:           "张三",
			Grade:          "2020级",
			Mobile:         &mobile,
			Position:       &position,
			MailingAddress: &mailingAddress,
			Status:         "active",
		},
	}
	svc := NewAlumniService(store, nil)

	detail, err := svc.GetByID(context.Background(), alumniID, common.AccessContext{Role: common.RoleAlumni, AlumniID: &alumniID})
	if err != nil {
		t.Fatalf("expected detail success, got %v", err)
	}
	if detail.Mobile == nil || *detail.Mobile != mobile {
		t.Fatalf("expected mobile %q, got %v", mobile, detail.Mobile)
	}
	if detail.Position == nil || *detail.Position != position {
		t.Fatalf("expected position %q, got %v", position, detail.Position)
	}
	if detail.MailingAddress == nil || *detail.MailingAddress != mailingAddress {
		t.Fatalf("expected mailing_address %q, got %v", mailingAddress, detail.MailingAddress)
	}
}

func TestAlumniServiceGetByIDShowsAllFieldsForAdmin(t *testing.T) {
	mobile := "13800000000"
	position := "主任"
	mailingAddress := "济南市"
	alumniID := uint64(9)
	store := &fakeAlumniStore{
		detail: &model.AlumniProfile{
			ID:             alumniID,
			Name:           "张三",
			Grade:          "2020级",
			Mobile:         &mobile,
			Position:       &position,
			MailingAddress: &mailingAddress,
			Status:         "active",
		},
	}
	svc := NewAlumniService(store, nil)

	detail, err := svc.GetByID(context.Background(), alumniID, common.AccessContext{Role: common.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("expected detail success, got %v", err)
	}
	if detail.Mobile == nil || *detail.Mobile != mobile {
		t.Fatalf("expected mobile %q, got %v", mobile, detail.Mobile)
	}
	if detail.Position == nil || *detail.Position != position {
		t.Fatalf("expected position %q, got %v", position, detail.Position)
	}
	if detail.MailingAddress == nil || *detail.MailingAddress != mailingAddress {
		t.Fatalf("expected mailing_address %q, got %v", mailingAddress, detail.MailingAddress)
	}
}

func TestAlumniServiceGetMeUsesBoundAlumniID(t *testing.T) {
	alumniID := uint64(9)
	store := &fakeAlumniStore{
		detail: &model.AlumniProfile{
			ID:     alumniID,
			Name:   "张三",
			Grade:  "2020级",
			Status: "active",
		},
	}
	svc := NewAlumniService(store, nil)

	detail, err := svc.GetMe(context.Background(), common.AccessContext{UserID: 3, Role: common.RoleAlumni, AlumniID: &alumniID})
	if err != nil {
		t.Fatalf("expected me success, got %v", err)
	}
	if store.detailID != alumniID {
		t.Fatalf("expected detail id %d, got %d", alumniID, store.detailID)
	}
	if detail.ID != alumniID {
		t.Fatalf("unexpected alumni detail: %+v", detail)
	}
}

func TestAlumniServiceGetMeRejectsNonAlumniAccess(t *testing.T) {
	svc := NewAlumniService(&fakeAlumniStore{}, nil)

	_, err := svc.GetMe(context.Background(), common.AccessContext{UserID: 3, Role: common.RoleAdmin})
	if err != common.ErrPermissionDenied {
		t.Fatalf("expected permission denied, got %v", err)
	}
}

func TestAlumniServiceGetMeRejectsUnboundAlumniAccess(t *testing.T) {
	svc := NewAlumniService(&fakeAlumniStore{}, nil)

	_, err := svc.GetMe(context.Background(), common.AccessContext{UserID: 3, Role: common.RoleAlumni})
	if err != common.ErrAlumniProfileUnbound {
		t.Fatalf("expected alumni profile unbound, got %v", err)
	}
}

func TestAlumniServiceUpdateMeUpdatesOnlyEditableFields(t *testing.T) {
	alumniID := uint64(9)
	workUnit := " 山东大学 "
	position := "主任"
	mobile := " 13800000000 "
	store := &fakeAlumniStore{
		detail: &model.AlumniProfile{
			ID:     alumniID,
			Name:   "张三",
			Grade:  "2020级",
			Status: "active",
		},
	}
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

func TestAlumniServiceDeleteSuccess(t *testing.T) {
	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil)

	err := svc.Delete(context.Background(), common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin}, 9)
	if err != nil {
		t.Fatalf("expected delete success, got %v", err)
	}
	if store.deleteID != 9 || store.deleteUserID != 7 {
		t.Fatalf("unexpected delete target: alumni=%d user=%d", store.deleteID, store.deleteUserID)
	}
}

func TestAlumniServiceDeleteReturnsNotFound(t *testing.T) {
	store := &fakeAlumniStore{deleteErr: common.ErrAlumniNotFound}
	svc := NewAlumniService(store, nil)

	err := svc.Delete(context.Background(), common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin}, 9)
	if err != common.ErrAlumniNotFound {
		t.Fatalf("expected alumni not found, got %v", err)
	}
}

func TestAlumniServiceListScopesDomainAndSensitiveSearch(t *testing.T) {
	store := &fakeAlumniStore{total: 1}
	svc := NewAlumniService(store, nil)
	viewer := common.AccessContext{
		Role:      common.RoleAdmin,
		DomainIDs: []uint64{2},
	}

	_, err := svc.List(context.Background(), dto.AlumniListRequest{Keyword: "张三"}, viewer)
	if err != nil {
		t.Fatalf("expected scoped list success, got %v", err)
	}
	if !slices.Equal(store.query.DataDomainIDs, []uint64{2}) {
		t.Fatalf("expected query to be limited to domain 2, got %v", store.query.DataDomainIDs)
	}
	if store.query.CanReadSensitive {
		t.Fatal("expected sensitive fields to remain unavailable")
	}

	_, err = svc.List(context.Background(), dto.AlumniListRequest{Position: "主任"}, viewer)
	if err != common.ErrPermissionDenied {
		t.Fatalf("expected sensitive position filter to be rejected, got %v", err)
	}
}

func TestAlumniServiceListHonorsRequestedDataDomain(t *testing.T) {
	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil)
	requestedDomainID := uint64(3)
	viewer := common.AccessContext{
		Role:      common.RoleAdmin,
		DomainIDs: []uint64{2, 3},
	}

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
	_, err := svc.List(context.Background(), dto.AlumniListRequest{}, common.AccessContext{Role: common.RoleAdmin})
	if err != common.ErrPermissionDenied {
		t.Fatalf("expected empty data-domain scope to be rejected, got %v", err)
	}
}

func TestAlumniServiceListMasksSensitiveFieldsWithoutPermission(t *testing.T) {
	mobile := "13800000000"
	email := "zhangsan@example.com"
	position := "主任"
	store := &fakeAlumniStore{
		items: []*model.AlumniProfile{{
			ID:       9,
			Name:     "张三",
			Grade:    "2020级",
			Mobile:   &mobile,
			Email:    &email,
			Position: &position,
		}},
		total: 1,
	}
	svc := NewAlumniService(store, nil)

	pager, err := svc.List(context.Background(), dto.AlumniListRequest{}, common.AccessContext{
		Role:      common.RoleAdmin,
		DomainIDs: []uint64{1},
	})
	if err != nil {
		t.Fatalf("expected list success, got %v", err)
	}
	item := pager.Items[0]
	if item.Mobile != nil || item.Email != nil || item.Position != nil {
		t.Fatalf("expected sensitive fields to be masked, got %+v", item)
	}
}

func TestAlumniServiceDetailUpdateAndDeletePassDomainScope(t *testing.T) {
	store := &fakeAlumniStore{
		detail:       &model.AlumniProfile{ID: 9, Name: "张三", Grade: "2020级", Status: common.AlumniStatusActive},
		updateResult: &model.AlumniProfile{ID: 9, Name: "张三", Grade: "2020级", Status: common.AlumniStatusActive},
	}
	svc := NewAlumniService(store, nil)
	viewer := common.AccessContext{UserID: 7, Role: common.RoleAdmin, DomainIDs: []uint64{1, 3}}

	if _, err := svc.GetByID(context.Background(), 9, viewer); err != nil {
		t.Fatalf("expected detail success, got %v", err)
	}
	if !slices.Equal(store.getDomainIDs, []uint64{1, 3}) {
		t.Fatalf("expected detail domain scope, got %v", store.getDomainIDs)
	}

	if _, err := svc.Update(context.Background(), viewer, 9, dto.AdminAlumniUpdateRequest{Name: "张三", Grade: "2020级"}); err != nil {
		t.Fatalf("expected update success, got %v", err)
	}
	if !slices.Equal(store.updateDomainIDs, []uint64{1, 3}) {
		t.Fatalf("expected update domain scope, got %v", store.updateDomainIDs)
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
	mobile := "13800000000"
	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil)

	_, err := svc.Create(context.Background(), common.AccessContext{
		UserID:    7,
		Role:      common.RoleAdmin,
		DomainIDs: []uint64{allowedDomainID},
	}, dto.AdminAlumniCreateRequest{
		DataDomainID: &forbiddenDomainID,
		Name:         "张三",
		Grade:        "2020级",
	})
	if err != nil {
		t.Fatalf("expected single-domain create success, got %v", err)
	}
	if store.createProfile.DataDomainID != allowedDomainID {
		t.Fatalf("expected forced domain %d, got %d", allowedDomainID, store.createProfile.DataDomainID)
	}

	_, err = svc.Create(context.Background(), common.AccessContext{
		UserID:    8,
		Role:      common.RoleAdmin,
		DomainIDs: []uint64{1, 2},
	}, dto.AdminAlumniCreateRequest{
		DataDomainID: &forbiddenDomainID,
		Name:         "李四",
		Grade:        "2020级",
		Mobile:       &mobile,
	})
	if err != common.ErrPermissionDenied {
		t.Fatalf("expected sensitive write to be rejected, got %v", err)
	}

	_, err = svc.Create(context.Background(), common.AccessContext{
		UserID:      8,
		Role:        common.RoleAdmin,
		DomainIDs:   []uint64{1, 2},
		Permissions: map[string]bool{common.PermissionAlumniSensitiveRead: true},
	}, dto.AdminAlumniCreateRequest{
		DataDomainID: &forbiddenDomainID,
		Name:         "王五",
		Grade:        "2020级",
	})
	if err != nil {
		t.Fatalf("expected multi-domain create in allowed domain, got %v", err)
	}
	if store.createProfile.DataDomainID != forbiddenDomainID {
		t.Fatalf("expected selected allowed domain %d, got %d", forbiddenDomainID, store.createProfile.DataDomainID)
	}
}

func TestAlumniServiceRejectsSensitiveUpdateWithoutPermission(t *testing.T) {
	position := "主任"
	svc := NewAlumniService(&fakeAlumniStore{}, nil)

	_, err := svc.Update(context.Background(), common.AccessContext{
		UserID:    7,
		Role:      common.RoleAdmin,
		DomainIDs: []uint64{1},
	}, 9, dto.AdminAlumniUpdateRequest{
		Name:     "张三",
		Grade:    "2020级",
		Position: &position,
	})
	if err != common.ErrPermissionDenied {
		t.Fatalf("expected sensitive update to be rejected, got %v", err)
	}
}

func TestAlumniServiceRejectsAlumniAsAdminOperator(t *testing.T) {
	svc := NewAlumniService(&fakeAlumniStore{}, nil)
	operator := common.AccessContext{UserID: 7, Role: common.RoleAlumni}

	if _, err := svc.Create(context.Background(), operator, dto.AdminAlumniCreateRequest{Name: "张三", Grade: "2020级"}); err != common.ErrPermissionDenied {
		t.Fatalf("expected create to reject alumni operator, got %v", err)
	}
	if _, err := svc.Update(context.Background(), operator, 9, dto.AdminAlumniUpdateRequest{Name: "张三", Grade: "2020级"}); err != common.ErrPermissionDenied {
		t.Fatalf("expected update to reject alumni operator, got %v", err)
	}
	if err := svc.Delete(context.Background(), operator, 9); err != common.ErrPermissionDenied {
		t.Fatalf("expected delete to reject alumni operator, got %v", err)
	}
}

func (s *fakeAlumniStore) FindByMobile(_ context.Context, _ string) (*model.AlumniProfile, error) {
	return s.profile, s.findErr
}

func (s *fakeAlumniStore) FindByEmail(_ context.Context, _ string) (*model.AlumniProfile, error) {
	return s.profile, s.findErr
}

func (s *fakeAlumniStore) UpdateMobile(_ context.Context, _ uint64, _ string) error {
	return s.updateErr
}

func (s *fakeAlumniStore) UpdateEmail(_ context.Context, _ uint64, _ string) error {
	return s.updateErr
}
