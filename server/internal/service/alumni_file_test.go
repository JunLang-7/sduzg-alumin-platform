package service

import (
	"context"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
)

func TestAlumniFileServiceGetAccessibleAlumniScopesDomain(t *testing.T) {
	store := &fakeAlumniStore{
		detail: &model.AlumniProfile{ID: 9, Name: "张三", Grade: "2020级", Status: common.AlumniStatusActive},
	}
	service := &AlumniFileService{alumni: store}

	_, err := service.getAccessibleAlumni(context.Background(), 9, common.AccessContext{
		Role:      common.RoleAdmin,
		DomainIDs: []uint64{2},
	})
	if err != nil {
		t.Fatalf("expected scoped alumni lookup success, got %v", err)
	}
	if len(store.getDomainIDs) != 1 || store.getDomainIDs[0] != 2 {
		t.Fatalf("expected file lookup to receive domain 2, got %v", store.getDomainIDs)
	}

	_, err = service.getAccessibleAlumni(context.Background(), 9, common.AccessContext{Role: common.RoleAdmin})
	if err != common.ErrPermissionDenied {
		t.Fatalf("expected empty data-domain scope to be rejected, got %v", err)
	}

	_, err = service.getAccessibleAlumni(context.Background(), 9, common.AccessContext{Role: common.RoleAlumni})
	if err != common.ErrPermissionDenied {
		t.Fatalf("expected non-admin file access to be rejected, got %v", err)
	}
}
