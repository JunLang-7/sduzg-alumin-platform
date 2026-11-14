package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/repository"
)

type fakeAlumniFileStore struct {
	file               *model.AlumniFile
	files              []*model.AlumniFile
	deletedID          uint64
	markDraftID        uint64
	deleteDraftErr     error
	deleteEnrollmentID uint64
	deleteOperatorID   uint64
}

func (s *fakeAlumniFileStore) Create(context.Context, *model.AlumniFile) (*model.AlumniFile, error) {
	return nil, nil
}

func (s *fakeAlumniFileStore) ListByEnrollmentID(ctx context.Context, enrollmentID uint64, includeDrafts bool) ([]*model.AlumniFile, error) {
	return s.ListByEnrollmentIDs(ctx, []uint64{enrollmentID}, includeDrafts)
}

func (s *fakeAlumniFileStore) ListByEnrollmentIDs(context.Context, []uint64, bool) ([]*model.AlumniFile, error) {
	if s.file != nil {
		return []*model.AlumniFile{s.file}, nil
	}
	return s.files, nil
}

func (s *fakeAlumniFileStore) GetByID(context.Context, uint64) (*model.AlumniFile, error) {
	return s.file, nil
}

func (s *fakeAlumniFileStore) GetByIDAnyStatus(context.Context, uint64) (*model.AlumniFile, error) {
	return s.file, nil
}

func (s *fakeAlumniFileStore) MarkDraft(_ context.Context, id uint64, _ uint64) error {
	s.markDraftID = id
	return nil
}

func (s *fakeAlumniFileStore) DeleteDraft(_ context.Context, id, enrollmentID, operatorID uint64) error {
	s.deleteEnrollmentID, s.deleteOperatorID = enrollmentID, operatorID
	if s.deleteDraftErr != nil {
		return s.deleteDraftErr
	}
	s.deletedID = id
	return nil
}

func (s *fakeAlumniFileStore) SoftDeleteByEnrollmentIDAndType(context.Context, uint64, string) error {
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

func fileTestAccessContext() common.AccessContext {
	return common.AccessContext{
		Role:        common.RoleAdmin,
		DomainIDs:   []uint64{2},
		Permissions: map[string]bool{common.PermissionAlumniFilesManage: true},
	}
}

func fileTestEnrollment() *model.AlumniEnrollment {
	return &model.AlumniEnrollment{
		ID: 11, AlumniID: 9, DataDomainID: 2, TrainingLevel: common.TrainingLevelMaster,
		Grade: "2020级", Status: common.AlumniStatusActive, Version: 1,
	}
}

func TestAlumniFileServiceGetAccessibleEnrollmentScopesDomain(t *testing.T) {
	store := &fakeAlumniStore{
		detail:            &model.AlumniProfile{ID: 9, Name: "张三", Status: common.AlumniStatusActive},
		detailEnrollments: []*model.AlumniEnrollment{fileTestEnrollment()},
	}
	service := &AlumniFileService{alumni: store}

	_, err := service.getAccessibleEnrollment(context.Background(), fileTestAccessContext(), 9, 11)
	if err != nil {
		t.Fatalf("expected scoped enrollment lookup success, got %v", err)
	}
	if len(store.getDomainIDs) != 1 || store.getDomainIDs[0] != 2 {
		t.Fatalf("expected file lookup to receive domain 2, got %v", store.getDomainIDs)
	}

	_, err = service.getAccessibleEnrollment(context.Background(), common.AccessContext{
		Role: common.RoleAdmin, Permissions: map[string]bool{common.PermissionAlumniFilesManage: true},
	}, 9, 11)
	if err != common.ErrPermissionDenied {
		t.Fatalf("expected empty data-domain scope to be rejected, got %v", err)
	}

	_, err = service.getAccessibleEnrollment(context.Background(), common.AccessContext{Role: common.RoleAlumni}, 9, 11)
	if err != common.ErrPermissionDenied {
		t.Fatalf("expected non-admin file access to be rejected, got %v", err)
	}
}

func TestAlumniFileServiceRequiresFileManagementPermission(t *testing.T) {
	store := &fakeAlumniStore{
		detail:            &model.AlumniProfile{ID: 9, Name: "张三", Status: common.AlumniStatusActive},
		detailEnrollments: []*model.AlumniEnrollment{fileTestEnrollment()},
	}
	service := &AlumniFileService{alumni: store}

	_, err := service.getAccessibleEnrollment(context.Background(), common.AccessContext{
		Role:        common.RoleAdmin,
		DomainIDs:   []uint64{2},
		Permissions: map[string]bool{common.PermissionAlumniSensitiveRead: true},
	}, 9, 11)
	if err != common.ErrPermissionDenied {
		t.Fatalf("expected sensitive-data permission not to grant file access, got %v", err)
	}
	if store.detailID != 0 {
		t.Fatalf("expected file permission to be checked before enrollment lookup, got lookup for %d", store.detailID)
	}
}

func TestAlumniFileServiceRejectsForgedEnrollmentAndFileCombination(t *testing.T) {
	enrollmentID := uint64(11)
	alumniStore := &fakeAlumniStore{
		detail:            &model.AlumniProfile{ID: 9, Name: "张三", Status: common.AlumniStatusActive},
		detailEnrollments: []*model.AlumniEnrollment{fileTestEnrollment()},
	}
	fileStore := &fakeAlumniFileStore{file: &model.AlumniFile{
		ID: 21, EnrollmentID: new(uint64(99)), Status: common.FileStatusDraft,
	}}
	service := &AlumniFileService{alumni: alumniStore, files: fileStore}

	err := service.DeleteDraftFile(context.Background(), fileTestAccessContext(), 9, enrollmentID, 21)
	if err != common.ErrFileNotFound {
		t.Fatalf("expected mismatched enrollment and file ids to be rejected, got %v", err)
	}
}

func TestAlumniFileServiceDeleteDraftFileOnlyRemovesUnsavedFiles(t *testing.T) {
	enrollmentID := uint64(11)
	alumniStore := &fakeAlumniStore{
		detail:            &model.AlumniProfile{ID: 9, Name: "张三", Status: common.AlumniStatusActive},
		detailEnrollments: []*model.AlumniEnrollment{fileTestEnrollment()},
	}
	fileStore := &fakeAlumniFileStore{file: &model.AlumniFile{
		ID: 21, EnrollmentID: &enrollmentID, Status: common.FileStatusActive,
	}}
	service := &AlumniFileService{alumni: alumniStore, files: fileStore}

	err := service.DeleteDraftFile(context.Background(), fileTestAccessContext(), 9, enrollmentID, 21)
	if err != common.ErrFileNotDraft {
		t.Fatalf("expected active file deletion to be rejected, got %v", err)
	}
	if fileStore.deletedID != 0 {
		t.Fatalf("active file must not be deleted by draft cleanup, got %d", fileStore.deletedID)
	}
}

func TestAlumniFileServiceListFilesGroupsByAccessibleEnrollment(t *testing.T) {
	enrollmentID := uint64(11)
	alumniStore := &fakeAlumniStore{
		detail:            &model.AlumniProfile{ID: 9, Name: "张三", Status: common.AlumniStatusActive},
		detailEnrollments: []*model.AlumniEnrollment{fileTestEnrollment()},
	}
	fileStore := &fakeAlumniFileStore{files: []*model.AlumniFile{
		{ID: 21, EnrollmentID: &enrollmentID, FileType: common.FileTypeDegreeArchive, OriginalName: "学位.pdf", Status: common.FileStatusActive},
		{ID: 22, EnrollmentID: &enrollmentID, FileType: common.FileTypeAcademicRecord, OriginalName: "学籍.pdf", Status: common.FileStatusActive},
	}}
	service := &AlumniFileService{alumni: alumniStore, files: fileStore}

	result, err := service.ListFiles(context.Background(), fileTestAccessContext(), 9, false)
	if err != nil {
		t.Fatalf("expected list files success, got %v", err)
	}
	if len(result.Enrollments) != 1 || result.Enrollments[0].EnrollmentID != enrollmentID {
		t.Fatalf("unexpected file groups: %+v", result.Enrollments)
	}
	if len(result.Enrollments[0].DegreeArchive) != 1 || len(result.Enrollments[0].AcademicRecord) != 1 {
		t.Fatalf("expected both file types to be grouped, got %+v", result.Enrollments[0])
	}
}

func TestAlumniFileServiceAuditIncludesEnrollmentScopeAndTargetDomain(t *testing.T) {
	writer := &fakeFileOperationLogger{}
	service := &AlumniFileService{opLogger: writer}
	operator := common.AccessContext{UserID: 7, Role: common.RoleAdmin, DomainIDs: []uint64{1, 3}}
	alumni := &model.AlumniProfile{ID: 9, Name: "张三"}
	enrollment := &model.AlumniEnrollment{ID: 11, AlumniID: 9, DataDomainID: 3}

	service.writeOpLog(context.Background(), operator, "download_alumni_file", 11, alumni, enrollment, common.FileTypeDegreeArchive, "证明.pdf")

	if len(writer.logs) != 1 {
		t.Fatalf("expected one audit log, got %d", len(writer.logs))
	}
	log := writer.logs[0]
	if log.OperatorID != 7 || log.OperatorRole != common.RoleAdmin || log.Detail == nil {
		t.Fatalf("unexpected audit log: %+v", log)
	}
	var detail struct {
		EnrollmentID          uint64   `json:"enrollment_id"`
		TargetDataDomainID    uint64   `json:"target_data_domain_id"`
		OperatorDataDomainIDs []uint64 `json:"operator_data_domain_ids"`
	}
	if err := json.Unmarshal([]byte(*log.Detail), &detail); err != nil {
		t.Fatalf("unmarshal audit detail: %v", err)
	}
	if detail.EnrollmentID != 11 || detail.TargetDataDomainID != 3 {
		t.Fatalf("unexpected audit target detail: %+v", detail)
	}
	if len(detail.OperatorDataDomainIDs) != 2 || detail.OperatorDataDomainIDs[0] != 1 || detail.OperatorDataDomainIDs[1] != 3 {
		t.Fatalf("unexpected audit access detail: %+v", detail)
	}
}

func TestAlumniFileServiceListDraftsIsolatesUploader(t *testing.T) {
	for _, includeDrafts := range []bool{false, true} {
		t.Run(strconv.FormatBool(includeDrafts), func(t *testing.T) {
			operator := fileTestAccessContext()
			operator.UserID = 7
			enrollmentID := uint64(11)
			files := &fakeAlumniFileStore{files: []*model.AlumniFile{
				{ID: 1, EnrollmentID: &enrollmentID, Status: common.FileStatusActive, FileType: common.FileTypeDegreeArchive, UploadedBy: new(uint64(99))},
				{ID: 2, EnrollmentID: &enrollmentID, Status: common.FileStatusDraft, FileType: common.FileTypeDegreeArchive, UploadedBy: new(uint64(7))},
				{ID: 3, EnrollmentID: &enrollmentID, Status: common.FileStatusDraft, FileType: common.FileTypeDegreeArchive, UploadedBy: new(uint64(99))},
				{ID: 4, EnrollmentID: &enrollmentID, Status: common.FileStatusDraft, FileType: common.FileTypeDegreeArchive},
				{ID: 5, EnrollmentID: &enrollmentID, Status: common.FileStatusPending, FileType: common.FileTypeDegreeArchive, UploadedBy: new(uint64(7))},
			}}
			service := &AlumniFileService{files: files, alumni: &fakeAlumniStore{detail: &model.AlumniProfile{ID: 9}, detailEnrollments: []*model.AlumniEnrollment{fileTestEnrollment()}}}
			result, err := service.ListFiles(context.Background(), operator, 9, includeDrafts)
			if err != nil {
				t.Fatal(err)
			}
			want := 1
			if includeDrafts {
				want = 2
			}
			if got := len(result.Enrollments[0].DegreeArchive); got != want {
				t.Fatalf("files = %+v, want %d", result.Enrollments[0].DegreeArchive, want)
			}
			for _, file := range result.Enrollments[0].DegreeArchive {
				if file.ID > 2 {
					t.Fatalf("leaked another uploader's or incomplete draft: %+v", file)
				}
			}
		})
	}
}

func TestAlumniFileServiceCancelDoesNotDeleteObjectAfterConcurrentActivation(t *testing.T) {
	operator := fileTestAccessContext()
	operator.UserID = 7
	fileStore := &fakeAlumniFileStore{file: &model.AlumniFile{ID: 21, EnrollmentID: new(uint64(11)), Status: common.FileStatusDraft, UploadedBy: new(uint64(7))}, deleteDraftErr: common.ErrFileNotFound}
	service := &AlumniFileService{files: fileStore, alumni: &fakeAlumniStore{detail: &model.AlumniProfile{ID: 9}, detailEnrollments: []*model.AlumniEnrollment{fileTestEnrollment()}}}
	// The atomic delete loses to activation. A nil object store also proves no blob deletion is attempted.
	err := service.DeleteDraftFile(context.Background(), operator, 9, 11, 21)
	if !errors.Is(err, common.ErrFileNotFound) {
		t.Fatalf("cancel after activation = %v", err)
	}
	if fileStore.deletedID != 0 || fileStore.deleteEnrollmentID != 11 || fileStore.deleteOperatorID != 7 {
		t.Fatalf("incorrect conditional cleanup: %+v", fileStore)
	}
}
