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
	updateID      uint64
	updateUserID  uint64
	updateProfile do.AlumniEditableProfile
	updateErr     error
}

func (s *fakeAlumniStore) List(_ context.Context, query do.AlumniListQuery) ([]*model.AlumniProfile, int64, error) {
	s.query = query
	return s.items, s.total, s.err
}

func (s *fakeAlumniStore) GetByID(_ context.Context, id uint64) (*model.AlumniProfile, error) {
	s.detailID = id
	return s.detail, s.detailErr
}

func (s *fakeAlumniStore) UpdateEditableFields(_ context.Context, id uint64, updaterID uint64, profile do.AlumniEditableProfile) error {
	s.updateID = id
	s.updateUserID = updaterID
	s.updateProfile = profile
	return s.updateErr
}

func TestAlumniServiceListNormalizesAndMapsItems(t *testing.T) {
	workUnit := "山东大学"
	position := "主任"
	updatedAt := time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC)
	store := &fakeAlumniStore{
		items: []*model.AlumniProfile{
			{
				ID:        9,
				Name:      "张三",
				Grade:     "2020级",
				WorkUnit:  &workUnit,
				Position:  &position,
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
	svc := NewAlumniService(store, nil)

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
			Role:     "alumni",
			AlumniID: &alumniID,
		},
	}
	svc := NewAlumniService(store, users)

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
			ID:   3,
			Role: "admin",
		},
	}
	svc := NewAlumniService(&fakeAlumniStore{}, users)

	_, err := svc.GetMe(context.Background(), 3)
	if err != common.ErrPermissionDenied {
		t.Fatalf("expected permission denied, got %v", err)
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
			Role:     "alumni",
			AlumniID: &alumniID,
		},
	}
	svc := NewAlumniService(store, users)

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
