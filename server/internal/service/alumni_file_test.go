package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/repository"
)

type fakeAlumniFileStore struct {
	file *model.AlumniFile
}

func (s *fakeAlumniFileStore) Create(context.Context, *model.AlumniFile) (*model.AlumniFile, error) {
	return nil, nil
}

func (s *fakeAlumniFileStore) ListByAlumniID(context.Context, uint64) ([]*model.AlumniFile, error) {
	return nil, nil
}

func (s *fakeAlumniFileStore) GetByID(context.Context, uint64) (*model.AlumniFile, error) {
	return s.file, nil
}

func (s *fakeAlumniFileStore) GetByIDAnyStatus(context.Context, uint64) (*model.AlumniFile, error) {
	return s.file, nil
}

func (s *fakeAlumniFileStore) MarkActive(context.Context, uint64, uint64) error { return nil }

func (s *fakeAlumniFileStore) SoftDelete(context.Context, uint64) error { return nil }

func (s *fakeAlumniFileStore) SoftDeleteByAlumniIDAndType(context.Context, uint64, string) error {
	return nil
}

func (s *fakeAlumniFileStore) SoftDeleteByAlumniID(context.Context, uint64) error { return nil }

var _ repository.AlumniFileStore = (*fakeAlumniFileStore)(nil)

type fakeFileOperationLogger struct {
	logs []*model.OperationLog
}

func (l *fakeFileOperationLogger) Write(_ context.Context, log *model.OperationLog) error {
	l.logs = append(l.logs, log)
	return nil
}

func TestAlumniFileServiceGetAccessibleAlumniScopesDomain(t *testing.T) {
	store := &fakeAlumniStore{
		detail: &model.AlumniProfile{ID: 9, Name: "张三", Grade: "2020级", Status: common.AlumniStatusActive},
	}
	service := &AlumniFileService{alumni: store}

	_, err := service.getAccessibleAlumni(context.Background(), 9, common.AccessContext{
		Role:        common.RoleAdmin,
		DomainIDs:   []uint64{2},
		Permissions: map[string]bool{common.PermissionAlumniFilesManage: true},
	})
	if err != nil {
		t.Fatalf("expected scoped alumni lookup success, got %v", err)
	}
	if len(store.getDomainIDs) != 1 || store.getDomainIDs[0] != 2 {
		t.Fatalf("expected file lookup to receive domain 2, got %v", store.getDomainIDs)
	}

	_, err = service.getAccessibleAlumni(context.Background(), 9, common.AccessContext{
		Role:        common.RoleAdmin,
		Permissions: map[string]bool{common.PermissionAlumniFilesManage: true},
	})
	if err != common.ErrPermissionDenied {
		t.Fatalf("expected empty data-domain scope to be rejected, got %v", err)
	}

	_, err = service.getAccessibleAlumni(context.Background(), 9, common.AccessContext{Role: common.RoleAlumni})
	if err != common.ErrPermissionDenied {
		t.Fatalf("expected non-admin file access to be rejected, got %v", err)
	}
}

func TestAlumniFileServiceRequiresFileManagementPermission(t *testing.T) {
	store := &fakeAlumniStore{
		detail: &model.AlumniProfile{ID: 9, Name: "张三", DataDomainID: 2, Status: common.AlumniStatusActive},
	}
	service := &AlumniFileService{alumni: store}

	_, err := service.getAccessibleAlumni(context.Background(), 9, common.AccessContext{
		Role:        common.RoleAdmin,
		DomainIDs:   []uint64{2},
		Permissions: map[string]bool{common.PermissionAlumniSensitiveRead: true},
	})
	if err != common.ErrPermissionDenied {
		t.Fatalf("expected sensitive-data permission not to grant file access, got %v", err)
	}
	if store.detailID != 0 {
		t.Fatalf("expected file permission to be checked before alumni lookup, got lookup for %d", store.detailID)
	}
}

func TestAlumniFileServiceRejectsForgedAlumniAndFileCombination(t *testing.T) {
	alumniStore := &fakeAlumniStore{
		detail: &model.AlumniProfile{ID: 9, Name: "张三", DataDomainID: 2, Status: common.AlumniStatusActive},
	}
	service := &AlumniFileService{
		alumni: alumniStore,
		files:  &fakeAlumniFileStore{file: &model.AlumniFile{ID: 11, AlumniID: 10}},
	}
	operator := common.AccessContext{
		Role:        common.RoleAdmin,
		DomainIDs:   []uint64{2},
		Permissions: map[string]bool{common.PermissionAlumniFilesManage: true},
	}

	err := service.DeleteFile(context.Background(), operator, 9, 11)
	if err != common.ErrFileNotFound {
		t.Fatalf("expected mismatched alumni and file ids to be rejected, got %v", err)
	}
}

func TestAlumniFileServiceAuditIncludesScopeAndTargetDomain(t *testing.T) {
	writer := &fakeFileOperationLogger{}
	service := &AlumniFileService{opLogger: writer}
	operator := common.AccessContext{UserID: 7, Role: common.RoleAdmin, DomainIDs: []uint64{1, 3}}
	alumni := &model.AlumniProfile{ID: 9, Name: "张三", DataDomainID: 3}

	service.writeOpLog(context.Background(), operator, "download_alumni_file", 11, alumni, common.FileTypeDegreeArchive, "证明.pdf")

	if len(writer.logs) != 1 {
		t.Fatalf("expected one audit log, got %d", len(writer.logs))
	}
	log := writer.logs[0]
	if log.OperatorID != 7 || log.OperatorRole != common.RoleAdmin || log.Detail == nil {
		t.Fatalf("unexpected audit log: %+v", log)
	}
	var detail struct {
		TargetDataDomainID    uint64   `json:"target_data_domain_id"`
		OperatorDataDomainIDs []uint64 `json:"operator_data_domain_ids"`
	}
	if err := json.Unmarshal([]byte(*log.Detail), &detail); err != nil {
		t.Fatalf("unmarshal audit detail: %v", err)
	}
	if detail.TargetDataDomainID != 3 || len(detail.OperatorDataDomainIDs) != 2 || detail.OperatorDataDomainIDs[0] != 1 || detail.OperatorDataDomainIDs[1] != 3 {
		t.Fatalf("unexpected audit access detail: %+v", detail)
	}
}
