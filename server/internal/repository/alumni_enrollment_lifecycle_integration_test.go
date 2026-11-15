package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func openLifecycleTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN is not configured")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func lifecycleDomains(t *testing.T, db *gorm.DB) (mpa, academic model.DataDomain) {
	t.Helper()
	if err := db.Where("code = ?", common.DataDomainMPA).First(&mpa).Error; err != nil {
		t.Fatalf("find MPA data domain: %v", err)
	}
	if err := db.Where("code = ?", common.DataDomainAcademicGraduate).First(&academic).Error; err != nil {
		t.Fatalf("find academic graduate data domain: %v", err)
	}
	return mpa, academic
}

func seedLifecycleAlumni(tx *gorm.DB, name string, enrollments ...*model.AlumniEnrollment) *model.AlumniProfile {
	profile := &model.AlumniProfile{Name: name, Status: common.AlumniStatusActive, Version: 1}
	if err := tx.Create(profile).Error; err != nil {
		panic(err)
	}
	for _, enrollment := range enrollments {
		enrollment.AlumniID = profile.ID
		if enrollment.Status == "" {
			enrollment.Status = common.AlumniStatusActive
		}
		if enrollment.Version == 0 {
			enrollment.Version = 1
		}
		if err := tx.Create(enrollment).Error; err != nil {
			panic(err)
		}
	}
	return profile
}

func TestAlumniRepositorySoftDeletesSingleEnrollmentAndKeepsFiles(t *testing.T) {
	db := openLifecycleTestDB(t)
	mpa, academic := lifecycleDomains(t, db)

	rollback := errors.New("rollback lifecycle test")
	err := db.Transaction(func(tx *gorm.DB) error {
		ctx := context.Background()
		repo := NewAlumniRepository(tx)
		profile := seedLifecycleAlumni(tx, "多段学籍删除测试",
			&model.AlumniEnrollment{DataDomainID: mpa.ID, TrainingLevel: common.TrainingLevelMaster, Grade: "2014"},
			&model.AlumniEnrollment{DataDomainID: academic.ID, TrainingLevel: common.TrainingLevelDoctor, Grade: "2020"},
		)
		var kept []model.AlumniEnrollment
		if err := tx.Where("alumni_id = ?", profile.ID).Order("id ASC").Find(&kept).Error; err != nil {
			return err
		}
		activeFile := &model.AlumniFile{
			EnrollmentID: &kept[1].ID, FileType: common.FileTypeDegreeArchive, ObjectKey: "lifecycle/kept.pdf",
			OriginalName: "博士学位.pdf", FileSize: 128, MimeType: "application/pdf", Status: common.FileStatusActive,
		}
		draftOwner := uint64(7)
		otherOwner := uint64(8)
		drafts := []*model.AlumniFile{
			{
				EnrollmentID: &kept[1].ID, FileType: common.FileTypeAcademicRecord, ObjectKey: "lifecycle/own-draft.pdf",
				OriginalName: "本人草稿.pdf", FileSize: 32, MimeType: "application/pdf",
				Status: common.FileStatusDraft, UploadedBy: &draftOwner,
			},
			{
				EnrollmentID: &kept[1].ID, FileType: common.FileTypeAcademicRecord, ObjectKey: "lifecycle/other-draft.pdf",
				OriginalName: "他人草稿.pdf", FileSize: 32, MimeType: "application/pdf",
				Status: common.FileStatusDraft, UploadedBy: &otherOwner,
			},
		}
		for _, file := range append([]*model.AlumniFile{activeFile}, drafts...) {
			if err := tx.Create(file).Error; err != nil {
				return err
			}
		}

		result, err := repo.Save(ctx, profile.ID, do.AlumniSaveInput{
			Version:  1,
			Personal: do.AlumniPersonalInput{Name: profile.Name},
			Enrollments: []do.AlumniEnrollmentInput{
				{ID: kept[0].ID, Version: kept[0].Version, DataDomainID: mpa.ID, TrainingLevel: common.TrainingLevelMaster, Grade: "2014级"},
				{ID: kept[1].ID, Version: kept[1].Version, Deleted: true},
			},
		}, 7, nil)
		if err != nil {
			return err
		}
		if !result.Visible || result.Record == nil || len(result.Record.Enrollments) != 1 {
			t.Fatalf("after delete visible=%t enrollments=%+v, want one remaining", result.Visible, result.Record)
		}
		if result.Record.Enrollments[0].ID != kept[0].ID {
			t.Fatalf("remaining enrollment = %d, want %d", result.Record.Enrollments[0].ID, kept[0].ID)
		}
		if len(result.RemovedObjectKeys) != 1 || result.RemovedObjectKeys[0] != "lifecycle/own-draft.pdf" {
			t.Fatalf("removed object keys = %v, want only the operator's own draft", result.RemovedObjectKeys)
		}

		var deleted model.AlumniEnrollment
		if err := tx.Unscoped().First(&deleted, kept[1].ID).Error; err != nil {
			return err
		}
		if deleted.Status != common.AlumniStatusDeleted || !deleted.DeletedAt.Valid {
			t.Fatalf("enrollment not soft deleted: %+v", deleted)
		}
		if deleted.Version != kept[1].Version+1 {
			t.Fatalf("deleted version = %d, want %d", deleted.Version, kept[1].Version+1)
		}

		var files []model.AlumniFile
		if err := tx.Unscoped().Where("enrollment_id = ?", kept[1].ID).Order("id ASC").Find(&files).Error; err != nil {
			return err
		}
		if len(files) != 3 {
			return errors.New("expected all enrollment files to be kept")
		}
		if files[0].Status != common.FileStatusActive || files[0].DeletedAt.Valid {
			t.Fatalf("active attachment must survive enrollment deletion: %+v", files[0])
		}
		if files[1].Status != common.FileStatusDeleted || !files[1].DeletedAt.Valid {
			t.Fatalf("operator draft should be cleaned up: %+v", files[1])
		}
		if files[2].Status != common.FileStatusDraft || files[2].DeletedAt.Valid {
			t.Fatalf("other operator's draft must be untouched: %+v", files[2])
		}

		var profileAfter model.AlumniProfile
		if err := tx.First(&profileAfter, profile.ID).Error; err != nil {
			return err
		}
		if profileAfter.Status != common.AlumniStatusActive || profileAfter.DeletedAt.Valid {
			t.Fatalf("profile must stay active: %+v", profileAfter)
		}

		if _, err := repo.GetDetail(ctx, profile.ID, nil); err != nil {
			return err
		}
		if _, err := repo.GetEnrollment(ctx, profile.ID, kept[1].ID, nil); !errors.Is(err, common.ErrEnrollmentNotFound) {
			t.Errorf("deleted enrollment should be unreadable, got %v", err)
		}
		rows, err := repo.ListAll(ctx, do.AlumniListQuery{})
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row.Profile != nil && row.Profile.ID == profile.ID && row.Enrollment != nil && row.Enrollment.ID == kept[1].ID {
				t.Errorf("soft-deleted enrollment leaked into export")
			}
		}

		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("transaction error = %v, want rollback sentinel", err)
	}
}

func TestAlumniRepositoryRejectsDeletingLastActiveEnrollment(t *testing.T) {
	db := openLifecycleTestDB(t)
	mpa, _ := lifecycleDomains(t, db)

	rollback := errors.New("rollback last enrollment test")
	err := db.Transaction(func(tx *gorm.DB) error {
		repo := NewAlumniRepository(tx)
		profile := seedLifecycleAlumni(tx, "最后一段学籍删除测试",
			&model.AlumniEnrollment{DataDomainID: mpa.ID, TrainingLevel: common.TrainingLevelMaster, Grade: "2016"},
		)
		var only model.AlumniEnrollment
		if err := tx.Where("alumni_id = ?", profile.ID).First(&only).Error; err != nil {
			return err
		}
		_, err := repo.Save(context.Background(), profile.ID, do.AlumniSaveInput{
			Version:  1,
			Personal: do.AlumniPersonalInput{Name: profile.Name},
			Enrollments: []do.AlumniEnrollmentInput{
				{ID: only.ID, Version: only.Version, Deleted: true},
			},
		}, 7, nil)
		if !errors.Is(err, common.ErrLastEnrollment) {
			t.Errorf("deleting the last active enrollment error = %v, want ErrLastEnrollment", err)
		}
		var reloaded model.AlumniEnrollment
		if err := tx.First(&reloaded, only.ID).Error; err != nil {
			return err
		}
		if reloaded.Status != common.AlumniStatusActive {
			t.Errorf("rollback failed, enrollment status = %q", reloaded.Status)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("transaction error = %v, want rollback sentinel", err)
	}
}

func TestAlumniRepositoryRejectsStaleEnrollmentVersionOnDelete(t *testing.T) {
	db := openLifecycleTestDB(t)
	mpa, academic := lifecycleDomains(t, db)

	rollback := errors.New("rollback stale delete test")
	err := db.Transaction(func(tx *gorm.DB) error {
		repo := NewAlumniRepository(tx)
		profile := seedLifecycleAlumni(tx, "并发删除测试",
			&model.AlumniEnrollment{DataDomainID: mpa.ID, TrainingLevel: common.TrainingLevelMaster, Grade: "2011"},
			&model.AlumniEnrollment{DataDomainID: academic.ID, TrainingLevel: common.TrainingLevelDoctor, Grade: "2018"},
		)
		var items []model.AlumniEnrollment
		if err := tx.Where("alumni_id = ?", profile.ID).Order("id ASC").Find(&items).Error; err != nil {
			return err
		}
		// 模拟另一名管理员已经保存过该段：数据库版本领先于前端提交的版本。
		if err := tx.Model(&model.AlumniEnrollment{}).Where("id = ?", items[1].ID).
			Update("version", items[1].Version+1).Error; err != nil {
			return err
		}
		_, err := repo.Save(context.Background(), profile.ID, do.AlumniSaveInput{
			Version:  1,
			Personal: do.AlumniPersonalInput{Name: profile.Name},
			Enrollments: []do.AlumniEnrollmentInput{
				{ID: items[1].ID, Version: items[1].Version, Deleted: true},
			},
		}, 7, nil)
		if !errors.Is(err, common.ErrVersionConflict) {
			t.Errorf("stale delete error = %v, want ErrVersionConflict", err)
		}
		var reloaded model.AlumniEnrollment
		if err := tx.First(&reloaded, items[1].ID).Error; err != nil {
			return err
		}
		if reloaded.Status != common.AlumniStatusActive {
			t.Errorf("enrollment deleted despite version conflict")
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("transaction error = %v, want rollback sentinel", err)
	}
}

func TestAlumniRepositoryCorrectsCategoryKeepingEnrollmentAndFiles(t *testing.T) {
	db := openLifecycleTestDB(t)
	mpa, academic := lifecycleDomains(t, db)

	rollback := errors.New("rollback category correction test")
	err := db.Transaction(func(tx *gorm.DB) error {
		repo := NewAlumniRepository(tx)
		profile := seedLifecycleAlumni(tx, "培养类别更正测试",
			&model.AlumniEnrollment{DataDomainID: mpa.ID, TrainingLevel: common.TrainingLevelMaster, Grade: "2013"},
		)
		var enrollment model.AlumniEnrollment
		if err := tx.Where("alumni_id = ?", profile.ID).First(&enrollment).Error; err != nil {
			return err
		}
		file := &model.AlumniFile{
			EnrollmentID: &enrollment.ID, FileType: common.FileTypeDegreeArchive, ObjectKey: "lifecycle/corrected.pdf",
			OriginalName: "学位.pdf", FileSize: 64, MimeType: "application/pdf", Status: common.FileStatusActive,
		}
		if err := tx.Create(file).Error; err != nil {
			return err
		}

		result, err := repo.Save(context.Background(), profile.ID, do.AlumniSaveInput{
			Version:  1,
			Personal: do.AlumniPersonalInput{Name: profile.Name},
			Enrollments: []do.AlumniEnrollmentInput{{
				ID: enrollment.ID, Version: enrollment.Version, DataDomainID: academic.ID,
				TrainingLevel: common.TrainingLevelMaster, Grade: "2013级",
			}},
		}, 7, nil)
		if err != nil {
			return err
		}
		if !result.Visible || len(result.Record.Enrollments) != 1 {
			t.Fatalf("unexpected correction result: %+v", result)
		}
		corrected := result.Record.Enrollments[0]
		if corrected.ID != enrollment.ID || corrected.DataDomainID != academic.ID {
			t.Fatalf("correction must keep the same enrollment id, got %+v", corrected)
		}
		if corrected.Version != enrollment.Version+1 {
			t.Fatalf("corrected version = %d, want %d", corrected.Version, enrollment.Version+1)
		}
		var reloadedFile model.AlumniFile
		if err := tx.First(&reloadedFile, file.ID).Error; err != nil {
			return err
		}
		if reloadedFile.Status != common.FileStatusActive || reloadedFile.EnrollmentID == nil || *reloadedFile.EnrollmentID != enrollment.ID {
			t.Fatalf("attachment association lost after category correction: %+v", reloadedFile)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("transaction error = %v, want rollback sentinel", err)
	}
}

func TestAlumniRepositoryHidesAlumniWhenOperatorLosesLastEnrollment(t *testing.T) {
	db := openLifecycleTestDB(t)
	mpa, academic := lifecycleDomains(t, db)

	rollback := errors.New("rollback hidden after save test")
	err := db.Transaction(func(tx *gorm.DB) error {
		repo := NewAlumniRepository(tx)
		profile := seedLifecycleAlumni(tx, "删除后不可见测试",
			&model.AlumniEnrollment{DataDomainID: mpa.ID, TrainingLevel: common.TrainingLevelMaster, Grade: "2015"},
			&model.AlumniEnrollment{DataDomainID: academic.ID, TrainingLevel: common.TrainingLevelUnverified, Grade: "2019"},
		)
		var items []model.AlumniEnrollment
		if err := tx.Where("alumni_id = ?", profile.ID).Order("id ASC").Find(&items).Error; err != nil {
			return err
		}
		result, err := repo.Save(context.Background(), profile.ID, do.AlumniSaveInput{
			Version:  1,
			Personal: do.AlumniPersonalInput{Name: profile.Name},
			Enrollments: []do.AlumniEnrollmentInput{
				{ID: items[0].ID, Version: items[0].Version, Deleted: true},
			},
		}, 7, []uint64{mpa.ID})
		if err != nil {
			return err
		}
		if result.Visible || result.Record == nil || len(result.Record.Enrollments) != 0 {
			t.Fatalf("expected successful save without accessible enrollments, got %+v", result)
		}
		if result.Record.Profile == nil || result.Record.Profile.ID != profile.ID {
			t.Fatalf("profile snapshot missing from hidden save result")
		}
		if _, err := repo.GetDetail(context.Background(), profile.ID, []uint64{mpa.ID}); !errors.Is(err, common.ErrAlumniNotFound) {
			t.Errorf("hidden alumni detail error = %v, want ErrAlumniNotFound", err)
		}
		// 隐藏学籍仍然存在，删除可见段不会触发“最后一段”限制。
		var hidden model.AlumniEnrollment
		if err := tx.First(&hidden, items[1].ID).Error; err != nil {
			return err
		}
		if hidden.Status != common.AlumniStatusActive {
			t.Errorf("hidden enrollment should stay active: %+v", hidden)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("transaction error = %v, want rollback sentinel", err)
	}
}

func TestAlumniRepositoryOrdersEnrollmentsAcrossReadPaths(t *testing.T) {
	db := openLifecycleTestDB(t)
	mpa, academic := lifecycleDomains(t, db)
	undergraduate := model.DataDomain{}
	if err := db.Where("code = ?", common.DataDomainUndergraduate).First(&undergraduate).Error; err != nil {
		t.Fatalf("find undergraduate data domain: %v", err)
	}

	rollback := errors.New("rollback ordering test")
	err := db.Transaction(func(tx *gorm.DB) error {
		repo := NewAlumniRepository(tx)
		profile := seedLifecycleAlumni(tx, "学籍排序测试",
			&model.AlumniEnrollment{DataDomainID: academic.ID, TrainingLevel: common.TrainingLevelDoctor, Grade: "2022"},
			&model.AlumniEnrollment{DataDomainID: mpa.ID, TrainingLevel: common.TrainingLevelMaster, Grade: "2018"},
			&model.AlumniEnrollment{DataDomainID: undergraduate.ID, TrainingLevel: common.TrainingLevelUndergraduate, Grade: "2014"},
			&model.AlumniEnrollment{DataDomainID: academic.ID, TrainingLevel: common.TrainingLevelUnverified, Grade: "未知"},
			&model.AlumniEnrollment{DataDomainID: mpa.ID, TrainingLevel: common.TrainingLevelMaster, Grade: ""},
		)
		record, err := repo.GetDetail(context.Background(), profile.ID, nil)
		if err != nil {
			return err
		}
		wantGrades := []string{"2014", "2018", "", "2022", "未知"}
		for index, enrollment := range record.Enrollments {
			if enrollment.Grade != wantGrades[index] {
				t.Fatalf("detail order = %v, want %v", gradesOf(record.Enrollments), wantGrades)
			}
		}
		rows, err := repo.ListAll(context.Background(), do.AlumniListQuery{})
		if err != nil {
			return err
		}
		var exported []string
		for _, row := range rows {
			if row.Profile != nil && row.Profile.ID == profile.ID && row.Enrollment != nil {
				exported = append(exported, row.Enrollment.Grade)
			}
		}
		if len(exported) != len(wantGrades) {
			t.Fatalf("export rows = %v, want %v", exported, wantGrades)
		}
		for index := range wantGrades {
			if exported[index] != wantGrades[index] {
				t.Fatalf("export order = %v, want %v", exported, wantGrades)
			}
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("transaction error = %v, want rollback sentinel", err)
	}
}

func gradesOf(enrollments []*model.AlumniEnrollment) []string {
	grades := make([]string, 0, len(enrollments))
	for _, enrollment := range enrollments {
		grades = append(grades, enrollment.Grade)
	}
	return grades
}

var _ = time.Now
