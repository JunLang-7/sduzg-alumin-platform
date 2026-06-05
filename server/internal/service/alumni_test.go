package service

import (
	"context"
	"testing"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
)

type fakeAlumniStore struct {
	query         do.AlumniListQuery
	items         []*model.AlumniProfile
	total         int64
	err           error
	detailID      uint64
	detail        *model.AlumniProfile
	detailErr     error
	createProfile *model.AlumniProfile
	createResult  *model.AlumniProfile
	createErr     error
	updateResult  *model.AlumniProfile
	updateID      uint64
	updateUserID  uint64
	updateProfile do.AlumniEditableProfile
	adminUpdate   do.AlumniUpdateProfile
	updateErr     error
	deleteID      uint64
	deleteUserID  uint64
	deleteErr     error
}

func (s *fakeAlumniStore) List(_ context.Context, query do.AlumniListQuery) ([]*model.AlumniProfile, int64, error) {
	s.query = query
	return s.items, s.total, s.err
}

func (s *fakeAlumniStore) GetByID(_ context.Context, id uint64) (*model.AlumniProfile, error) {
	s.detailID = id
	return s.detail, s.detailErr
}

func (s *fakeAlumniStore) Create(_ context.Context, profile *do.AlumniCreateProfile, operatorID uint64) (*model.AlumniProfile, error) {
	s.createProfile = &model.AlumniProfile{
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

func (s *fakeAlumniStore) Update(_ context.Context, id uint64, updaterID uint64, profile do.AlumniUpdateProfile) error {
	s.updateID = id
	s.updateUserID = updaterID
	s.adminUpdate = profile
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

func (s *fakeAlumniStore) Delete(_ context.Context, id uint64, updaterID uint64) error {
	s.deleteID = id
	s.deleteUserID = updaterID
	return s.deleteErr
}

func TestAlumniServiceCreateNormalizesAndMapsDetail(t *testing.T) {
	className := " 2020级MPA周末班 "
	emptyMentor := " "
	workUnit := " 山东大学 "
	store := &fakeAlumniStore{}
	svc := NewAlumniService(store, nil, nil)

	detail, err := svc.Create(context.Background(), 7, dto.AdminAlumniCreateRequest{
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
	svc := NewAlumniService(&fakeAlumniStore{}, nil, nil)

	_, err := svc.Create(context.Background(), 7, dto.AdminAlumniCreateRequest{
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
	svc := NewAlumniService(store, nil, nil)

	detail, err := svc.Update(context.Background(), 7, 9, dto.AdminAlumniUpdateRequest{
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
	svc := NewAlumniService(store, nil, nil)

	_, err := svc.Update(context.Background(), 7, 9, dto.AdminAlumniUpdateRequest{
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
	svc := NewAlumniService(store, nil, nil)

	pager, err := svc.List(context.Background(), dto.AlumniListRequest{
		Page:     0,
		PageSize: 1000,
		Keyword:  " 张三 ",
	})
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
	store := &fakeAlumniStore{
		detail: &model.AlumniProfile{
			ID:             9,
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
	svc := NewAlumniService(store, nil, nil)

	detail, err := svc.GetByID(context.Background(), 9)
	if err != nil {
		t.Fatalf("expected detail success, got %v", err)
	}
	if store.detailID != 9 {
		t.Fatalf("expected detail id 9, got %d", store.detailID)
	}
	if detail.ID != 9 || detail.Name != "张三" || detail.Counselor == nil || *detail.Counselor != counselor {
		t.Fatalf("unexpected alumni detail: %+v", detail)
	}
	if !detail.CreatedAt.Equal(createdAt) || !detail.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("unexpected detail times: %+v", detail)
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
	users := &fakeUserStore{
		user: &model.User{
			ID:       3,
			Role:     common.RoleAlumni,
			AlumniID: &alumniID,
			Status:   common.UserStatusActive,
		},
	}
	svc := NewAlumniService(store, users, nil)

	detail, err := svc.GetMe(context.Background(), 3)
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

func TestAlumniServiceGetMeRejectsNonAlumniUser(t *testing.T) {
	users := &fakeUserStore{
		user: &model.User{
			ID:     3,
			Role:   common.RoleAdmin,
			Status: common.UserStatusActive,
		},
	}
	svc := NewAlumniService(&fakeAlumniStore{}, users, nil)

	_, err := svc.GetMe(context.Background(), 3)
	if err != common.ErrPermissionDenied {
		t.Fatalf("expected permission denied, got %v", err)
	}
}

func TestAlumniServiceGetMeRejectsDisabledUser(t *testing.T) {
	alumniID := uint64(9)
	users := &fakeUserStore{
		user: &model.User{
			ID:       3,
			Role:     common.RoleAlumni,
			AlumniID: &alumniID,
			Status:   "disabled",
		},
	}
	svc := NewAlumniService(&fakeAlumniStore{}, users, nil)

	_, err := svc.GetMe(context.Background(), 3)
	if err != common.ErrAccountDisabled {
		t.Fatalf("expected account disabled, got %v", err)
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
	users := &fakeUserStore{
		user: &model.User{
			ID:       3,
			Role:     common.RoleAlumni,
			AlumniID: &alumniID,
			Status:   common.UserStatusActive,
		},
	}
	svc := NewAlumniService(store, users, nil)

	_, err := svc.UpdateMe(context.Background(), 3, dto.AlumniProfileUpdateRequest{
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
	svc := NewAlumniService(store, nil, nil)

	err := svc.Delete(context.Background(), 7, 9)
	if err != nil {
		t.Fatalf("expected delete success, got %v", err)
	}
	if store.deleteID != 9 || store.deleteUserID != 7 {
		t.Fatalf("unexpected delete target: alumni=%d user=%d", store.deleteID, store.deleteUserID)
	}
}

func TestAlumniServiceDeleteReturnsNotFound(t *testing.T) {
	store := &fakeAlumniStore{deleteErr: common.ErrAlumniNotFound}
	svc := NewAlumniService(store, nil, nil)

	err := svc.Delete(context.Background(), 7, 9)
	if err != common.ErrAlumniNotFound {
		t.Fatalf("expected alumni not found, got %v", err)
	}
}
