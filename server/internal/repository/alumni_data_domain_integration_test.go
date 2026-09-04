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

func TestAlumniRepositoryCreateAndBatchCreateDefaultToMPADataDomain(t *testing.T) {
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

	var mpa model.DataDomain
	if err := db.Where("code = ?", common.DataDomainMPA).First(&mpa).Error; err != nil {
		t.Fatalf("find MPA data domain: %v", err)
	}

	rollback := errors.New("rollback test transaction")
	err = db.Transaction(func(tx *gorm.DB) error {
		repo := NewAlumniRepository(tx)
		ctx := context.Background()

		created, err := repo.Create(ctx, &do.AlumniCreateProfile{
			Name:   "数据域单条新增测试",
			Grade:  "2026级",
			Status: common.AlumniStatusActive,
		}, 1)
		if err != nil {
			return err
		}
		if created.DataDomainID != mpa.ID {
			t.Errorf("Create() data domain id = %d, want MPA id %d", created.DataDomainID, mpa.ID)
		}

		if err := repo.BatchCreate(ctx, []do.AlumniCreateProfile{
			{Name: "数据域批量新增测试一", Grade: "2026级", Status: common.AlumniStatusActive},
			{Name: "数据域批量新增测试二", Grade: "2026级", Status: common.AlumniStatusActive},
		}, 1); err != nil {
			return err
		}

		var count int64
		if err := tx.Model(&model.AlumniProfile{}).
			Where("data_domain_id = ?", mpa.ID).
			Where("name IN ?", []string{"数据域批量新增测试一", "数据域批量新增测试二"}).
			Count(&count).
			Error; err != nil {
			return err
		}
		if count != 2 {
			t.Errorf("BatchCreate() created %d MPA records, want 2", count)
		}

		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("transaction error = %v, want rollback sentinel", err)
	}
}

func TestAlumniRepositoryScopesReadAndWriteByDataDomain(t *testing.T) {
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

	rollback := errors.New("rollback test transaction")
	err = db.Transaction(func(tx *gorm.DB) error {
		mpaProfile := &model.AlumniProfile{
			DataDomainID: mpa.ID,
			Name:         "数据域范围测试MPA",
			Grade:        "2099级数据域测试",
			Status:       common.AlumniStatusActive,
		}
		academicProfile := &model.AlumniProfile{
			DataDomainID: academic.ID,
			Name:         "数据域范围测试学硕",
			Grade:        "2099级数据域测试",
			Status:       common.AlumniStatusActive,
		}
		if err := tx.Create(mpaProfile).Error; err != nil {
			return err
		}
		if err := tx.Create(academicProfile).Error; err != nil {
			return err
		}

		repo := NewAlumniRepository(tx)
		ctx := context.Background()
		items, total, err := repo.List(ctx, do.AlumniListQuery{
			Grade:         "2099级数据域测试",
			DataDomainIDs: []uint64{mpa.ID},
		})
		if err != nil {
			return err
		}
		if total != 1 || len(items) != 1 || items[0].ID != mpaProfile.ID {
			t.Errorf("scoped list = total %d, items %+v; want only MPA profile %d", total, items, mpaProfile.ID)
		}

		if _, err := repo.GetByID(ctx, academicProfile.ID, []uint64{mpa.ID}); !errors.Is(err, common.ErrAlumniNotFound) {
			t.Errorf("cross-domain GetByID error = %v, want ErrAlumniNotFound", err)
		}
		if err := repo.Update(ctx, academicProfile.ID, 1, do.AlumniUpdateProfile{
			Name:  "不应更新",
			Grade: "2099级数据域测试",
		}, []uint64{mpa.ID}); !errors.Is(err, common.ErrAlumniNotFound) {
			t.Errorf("cross-domain Update error = %v, want ErrAlumniNotFound", err)
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
