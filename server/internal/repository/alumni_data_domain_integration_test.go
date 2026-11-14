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

func TestAlumniRepositoryCreateAndBatchCreateUseEnrollmentDataDomain(t *testing.T) {
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

	var mpa, academic model.DataDomain
	if err := db.Where("code = ?", common.DataDomainMPA).First(&mpa).Error; err != nil {
		t.Fatalf("find MPA data domain: %v", err)
	}
	if err := db.Where("code = ?", common.DataDomainAcademicGraduate).First(&academic).Error; err != nil {
		t.Fatalf("find academic graduate data domain: %v", err)
	}

	present := "2026级"
	rollback := errors.New("rollback test transaction")
	err = db.Transaction(func(tx *gorm.DB) error {
		repo := NewAlumniRepository(tx)
		ctx := context.Background()

		created, err := repo.Create(ctx, do.AlumniCreateInput{
			Personal: do.AlumniPersonalInput{Name: "数据域单条新增测试"},
			Enrollments: []do.AlumniEnrollmentInput{{
				DataDomainID:  mpa.ID,
				TrainingLevel: common.TrainingLevelMaster,
				Grade:         present,
			}},
		}, 1)
		if err != nil {
			return err
		}
		if len(created.Enrollments) != 1 || created.Enrollments[0].DataDomainID != mpa.ID {
			t.Errorf("Create() enrollment domain = %+v, want MPA id %d", created.Enrollments, mpa.ID)
		}

		if _, err := repo.BatchCreate(ctx, []do.AlumniCreateInput{
			{
				Personal: do.AlumniPersonalInput{Name: "数据域批量新增测试一"},
				Enrollments: []do.AlumniEnrollmentInput{{
					DataDomainID: mpa.ID, TrainingLevel: common.TrainingLevelMaster, Grade: present,
				}},
			},
			{
				Personal: do.AlumniPersonalInput{Name: "数据域批量新增测试二"},
				Enrollments: []do.AlumniEnrollmentInput{{
					DataDomainID: academic.ID, TrainingLevel: common.TrainingLevelUnverified, Grade: present,
				}},
			},
		}, 1); err != nil {
			return err
		}

		var mpaCount, academicCount int64
		if err := tx.Model(&model.AlumniEnrollment{}).
			Where("data_domain_id = ?", mpa.ID).
			Where("grade = ?", present).
			Where("alumni_id IN (SELECT id FROM alumni_profiles WHERE name IN ?)", []string{"数据域批量新增测试一", "数据域单条新增测试"}).
			Count(&mpaCount).Error; err != nil {
			return err
		}
		if mpaCount != 2 {
			t.Errorf("BatchCreate() created %d MPA enrollments, want 2", mpaCount)
		}
		if err := tx.Model(&model.AlumniEnrollment{}).
			Where("data_domain_id = ?", academic.ID).
			Where("grade = ?", present).
			Count(&academicCount).Error; err != nil {
			return err
		}
		if academicCount != 1 {
			t.Errorf("BatchCreate() created %d academic enrollments, want 1", academicCount)
		}

		unknownDomainID := uint64(999999999)
		if _, err := repo.Create(ctx, do.AlumniCreateInput{
			Personal: do.AlumniPersonalInput{Name: "不存在的数据域测试"},
			Enrollments: []do.AlumniEnrollmentInput{{
				DataDomainID: unknownDomainID, TrainingLevel: common.TrainingLevelMaster, Grade: present,
			}},
		}, 1); !errors.Is(err, common.ErrInvalidDataDomain) {
			t.Errorf("Create() invalid domain error = %v, want ErrInvalidDataDomain", err)
		}

		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("transaction error = %v, want rollback sentinel", err)
	}
}

func TestAlumniRepositoryScopesReadAndWriteByEnrollmentDataDomain(t *testing.T) {
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

	var mpa, academic model.DataDomain
	if err := db.Where("code = ?", common.DataDomainMPA).First(&mpa).Error; err != nil {
		t.Fatalf("find MPA data domain: %v", err)
	}
	if err := db.Where("code = ?", common.DataDomainAcademicGraduate).First(&academic).Error; err != nil {
		t.Fatalf("find academic graduate data domain: %v", err)
	}

	const grade = "2099级数据域测试"
	rollback := errors.New("rollback test transaction")
	err = db.Transaction(func(tx *gorm.DB) error {
		mpaProfile := &model.AlumniProfile{Name: "数据域范围测试MPA", Status: common.AlumniStatusActive, Version: 1}
		academicProfile := &model.AlumniProfile{Name: "数据域范围测试学硕", Status: common.AlumniStatusActive, Version: 1}
		if err := tx.Create(mpaProfile).Error; err != nil {
			return err
		}
		if err := tx.Create(academicProfile).Error; err != nil {
			return err
		}
		mpaEnrollment := &model.AlumniEnrollment{
			AlumniID: mpaProfile.ID, DataDomainID: mpa.ID, TrainingLevel: common.TrainingLevelMaster,
			Grade: grade, Status: common.AlumniStatusActive, Version: 1,
		}
		academicEnrollment := &model.AlumniEnrollment{
			AlumniID: academicProfile.ID, DataDomainID: academic.ID, TrainingLevel: common.TrainingLevelUnverified,
			Grade: grade, Status: common.AlumniStatusActive, Version: 1,
		}
		if err := tx.Create(mpaEnrollment).Error; err != nil {
			return err
		}
		if err := tx.Create(academicEnrollment).Error; err != nil {
			return err
		}

		repo := NewAlumniRepository(tx)
		ctx := context.Background()
		items, total, err := repo.List(ctx, do.AlumniListQuery{
			Grade:         grade,
			DataDomainIDs: []uint64{mpa.ID},
		})
		if err != nil {
			return err
		}
		if total != 1 || len(items) != 1 || items[0].Profile.ID != mpaProfile.ID {
			t.Errorf("scoped list = total %d, items %+v; want only MPA profile %d", total, items, mpaProfile.ID)
		}
		if len(items) == 1 && len(items[0].Enrollments) != 1 {
			t.Errorf("scoped list leaked %d enrollments, want 1", len(items[0].Enrollments))
		}

		if _, err := repo.GetDetail(ctx, academicProfile.ID, []uint64{mpa.ID}); !errors.Is(err, common.ErrAlumniNotFound) {
			t.Errorf("cross-domain GetDetail error = %v, want ErrAlumniNotFound", err)
		}
		if _, err := repo.Save(ctx, academicProfile.ID, do.AlumniSaveInput{
			Version:  academicProfile.Version,
			Personal: do.AlumniPersonalInput{Name: "不应更新"},
		}, 1, []uint64{mpa.ID}); !errors.Is(err, common.ErrAlumniNotFound) {
			t.Errorf("cross-domain Save error = %v, want ErrAlumniNotFound", err)
		}
		if err := repo.Delete(ctx, academicProfile.ID, 1, []uint64{mpa.ID}); !errors.Is(err, common.ErrAlumniNotFound) {
			t.Errorf("cross-domain Delete error = %v, want ErrAlumniNotFound", err)
		}

		var unchanged model.AlumniProfile
		if err := tx.First(&unchanged, academicProfile.ID).Error; err != nil {
			return err
		}
		if unchanged.Name != academicProfile.Name || unchanged.Status != common.AlumniStatusActive {
			t.Errorf("cross-domain profile was changed: %+v", unchanged)
		}

		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("transaction error = %v, want rollback sentinel", err)
	}
}

func TestAlumniRepositoryHidesOtherEnrollmentsAndRefusesMultiDelete(t *testing.T) {
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

	var mpa, undergraduate model.DataDomain
	if err := db.Where("code = ?", common.DataDomainMPA).First(&mpa).Error; err != nil {
		t.Fatalf("find MPA domain: %v", err)
	}
	if err := db.Where("code = ?", common.DataDomainUndergraduate).First(&undergraduate).Error; err != nil {
		t.Fatalf("find undergraduate domain: %v", err)
	}

	rollback := errors.New("rollback multi-enrollment test")
	err = db.Transaction(func(tx *gorm.DB) error {
		profile := &model.AlumniProfile{Name: "多段学籍权限测试", Status: common.AlumniStatusActive, Version: 1}
		if err := tx.Create(profile).Error; err != nil {
			return err
		}
		visible := &model.AlumniEnrollment{
			AlumniID: profile.ID, DataDomainID: mpa.ID, TrainingLevel: common.TrainingLevelMaster,
			Grade: "2019级", Status: common.AlumniStatusActive, Version: 1,
		}
		hidden := &model.AlumniEnrollment{
			AlumniID: profile.ID, DataDomainID: undergraduate.ID, TrainingLevel: common.TrainingLevelUndergraduate,
			Grade: "2015级", Status: common.AlumniStatusActive, Version: 1,
		}
		if err := tx.Create(visible).Error; err != nil {
			return err
		}
		if err := tx.Create(hidden).Error; err != nil {
			return err
		}

		repo := NewAlumniRepository(tx)
		ctx := context.Background()
		detail, err := repo.GetDetail(ctx, profile.ID, []uint64{mpa.ID})
		if err != nil {
			return err
		}
		if len(detail.Enrollments) != 1 || detail.Enrollments[0].ID != visible.ID {
			t.Errorf("detail leaked hidden enrollments: %+v", detail.Enrollments)
		}

		hiddenGradeItems, hiddenGradeTotal, err := repo.List(ctx, do.AlumniListQuery{Grade: "2015级", DataDomainIDs: []uint64{mpa.ID}})
		if err != nil {
			return err
		}
		if hiddenGradeTotal != 0 || len(hiddenGradeItems) != 0 {
			t.Errorf("hidden enrollment grade leaked into MPA list: total=%d items=%+v", hiddenGradeTotal, hiddenGradeItems)
		}

		if _, err := repo.Save(ctx, profile.ID, do.AlumniSaveInput{
			Version:  profile.Version,
			Personal: do.AlumniPersonalInput{Name: "不应保存"},
			Enrollments: []do.AlumniEnrollmentInput{{
				ID: hidden.ID, Version: hidden.Version, DataDomainID: undergraduate.ID,
				TrainingLevel: common.TrainingLevelUndergraduate, Grade: "2015级",
			}},
		}, 1, []uint64{mpa.ID}); !errors.Is(err, common.ErrEnrollmentNotFound) {
			t.Errorf("hidden enrollment save error = %v, want ErrEnrollmentNotFound", err)
		}

		if err := repo.Delete(ctx, profile.ID, 1, []uint64{mpa.ID}); !errors.Is(err, common.ErrAlumniMultiEnrollment) {
			t.Errorf("multi-enrollment delete error = %v, want ErrAlumniMultiEnrollment", err)
		}

		var untouched model.AlumniProfile
		if err := tx.First(&untouched, profile.ID).Error; err != nil {
			return err
		}
		if untouched.Name != profile.Name || untouched.Status != common.AlumniStatusActive {
			t.Errorf("multi-enrollment fixture was modified: %+v", untouched)
		}
		var hiddenAfter model.AlumniEnrollment
		if err := tx.First(&hiddenAfter, hidden.ID).Error; err != nil {
			return err
		}
		if hiddenAfter.Grade != "2015级" || hiddenAfter.Status != common.AlumniStatusActive {
			t.Errorf("hidden enrollment was modified: %+v", hiddenAfter)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("transaction error = %v, want rollback sentinel", err)
	}
}
