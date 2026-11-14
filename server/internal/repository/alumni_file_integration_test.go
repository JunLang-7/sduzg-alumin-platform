package repository

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Exercise both orderings at the cancellation's atomic write, including a stale
// service read before activation. All fixtures are rolled back after each case.
func TestAlumniFileRepositoryDraftCleanupAndSaveOrdering(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN is not configured")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	for _, scenario := range []string{"activation_first", "cancel_first", "pending", "wrong_uploader", "wrong_enrollment", "formal_remove"} {
		t.Run(scenario, func(t *testing.T) {
			rollback := errors.New("rollback fixtures")
			err := db.Transaction(func(tx *gorm.DB) error {
				ctx := context.Background()
				var domain model.DataDomain
				if err := tx.Where("code = ?", common.DataDomainMPA).First(&domain).Error; err != nil {
					return err
				}
				repo := NewAlumniRepository(tx)
				record, err := repo.Create(ctx, do.AlumniCreateInput{Personal: do.AlumniPersonalInput{Name: "附件事务回归"}, Enrollments: []do.AlumniEnrollmentInput{{DataDomainID: domain.ID, TrainingLevel: common.TrainingLevelMaster, Grade: "2026"}}}, 1)
				if err != nil {
					return err
				}
				enrollment := record.Enrollments[0]
				status := common.FileStatusDraft
				if scenario == "pending" {
					status = common.FileStatusPending
				}
				if scenario == "formal_remove" {
					status = common.FileStatusActive
				}
				file := &model.AlumniFile{EnrollmentID: &enrollment.ID, FileType: common.FileTypeDegreeArchive, OriginalName: "学位.pdf", ObjectKey: "review241/degree.pdf", UploadedBy: new(uint64(1)), Status: status}
				if err := tx.Create(file).Error; err != nil {
					return err
				}
				files := NewAlumniFileRepository(tx)
				// Simulate DeleteDraftFile having already fetched this snapshot.
				stale, err := files.GetByIDAnyStatus(ctx, file.ID)
				if err != nil {
					return err
				}
				input := do.AlumniSaveInput{Version: record.Profile.Version, Personal: do.AlumniPersonalInput{Name: record.Profile.Name}, Enrollments: []do.AlumniEnrollmentInput{{ID: enrollment.ID, Version: enrollment.Version, DataDomainID: domain.ID, TrainingLevel: enrollment.TrainingLevel, Grade: enrollment.Grade, ActivateFileIDs: []uint64{file.ID}}}}
				switch scenario {
				case "activation_first":
					result, err := repo.Save(ctx, record.Profile.ID, input, 1, []uint64{domain.ID})
					if err != nil {
						return err
					}
					if len(result.FileChanges) != 1 || result.FileChanges[0].Removed || result.FileChanges[0].OriginalName != file.OriginalName || result.FileChanges[0].EnrollmentID != enrollment.ID {
						t.Fatalf("missing committed activation audit: %+v", result)
					}
					if err := files.DeleteDraft(ctx, stale.ID, *stale.EnrollmentID, *stale.UploadedBy); !errors.Is(err, common.ErrFileNotFound) {
						t.Fatalf("stale cancellation deleted committed file: %v", err)
					}
				case "cancel_first":
					if err := files.DeleteDraft(ctx, file.ID, enrollment.ID, 1); err != nil {
						return err
					}
					result, err := repo.Save(ctx, record.Profile.ID, input, 1, []uint64{domain.ID})
					if !errors.Is(err, common.ErrFileNotFound) || result != nil {
						t.Fatalf("save after cancel = %+v, %v", result, err)
					}
					after, err := repo.GetDetail(ctx, record.Profile.ID, nil)
					if err != nil {
						return err
					}
					if after.Profile.Version != record.Profile.Version || after.Enrollments[0].Version != enrollment.Version {
						t.Fatal("failed attachment activation partially committed profile or enrollment")
					}
				case "pending":
					if err := files.DeleteDraft(ctx, file.ID, enrollment.ID, 1); err != nil {
						return err
					}
				case "wrong_uploader", "wrong_enrollment":
					operatorID, enrollmentID := uint64(1), enrollment.ID
					if scenario == "wrong_uploader" {
						operatorID = 999
					} else {
						enrollmentID++
					}
					if err := files.DeleteDraft(ctx, file.ID, enrollmentID, operatorID); !errors.Is(err, common.ErrFileNotFound) {
						t.Fatalf("invalid draft cleanup accepted: %v", err)
					}
				case "formal_remove":
					input.Enrollments[0].ActivateFileIDs = nil
					input.Enrollments[0].RemoveFileIDs = []uint64{file.ID}
					result, err := repo.Save(ctx, record.Profile.ID, input, 1, nil)
					if err != nil {
						return err
					}
					if len(result.FileChanges) != 1 || !result.FileChanges[0].Removed || result.FileChanges[0].FileID != file.ID || len(result.RemovedObjectKeys) != 1 || result.RemovedObjectKeys[0] != file.ObjectKey {
						t.Fatalf("missing committed removal audit/cleanup: %+v", result)
					}
				}
				var after model.AlumniFile
				if err := tx.Unscoped().First(&after, file.ID).Error; err != nil {
					return err
				}
				want := common.FileStatusDraft
				if scenario == "activation_first" {
					want = common.FileStatusActive
				}
				if scenario == "cancel_first" || scenario == "pending" || scenario == "formal_remove" {
					want = common.FileStatusDeleted
				}
				if after.Status != want {
					t.Fatalf("file status = %s, want %s", after.Status, want)
				}
				return rollback
			})
			if !errors.Is(err, rollback) {
				t.Fatal(err)
			}
		})
	}
}
