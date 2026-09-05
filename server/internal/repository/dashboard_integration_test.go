package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestDashboardRepositoryScopesStatisticsByDataDomain(t *testing.T) {
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
		repo := NewDashboardRepository(tx)
		ctx := context.Background()

		beforeMPA, err := repo.Overview(ctx, []uint64{mpa.ID})
		if err != nil {
			return err
		}
		beforeAcademic, err := repo.Overview(ctx, []uint64{academic.ID})
		if err != nil {
			return err
		}
		beforeBoth, err := repo.Overview(ctx, []uint64{mpa.ID, academic.ID})
		if err != nil {
			return err
		}

		suffix := time.Now().UnixNano()
		mpaGrade := fmt.Sprintf("dashboard-scope-mpa-%d", suffix)
		academicGrade := fmt.Sprintf("dashboard-scope-academic-%d", suffix)
		mobile := "13800000000"
		workUnit := "测试单位"
		mentor := "测试导师"
		mpaComplete := &model.AlumniProfile{
			DataDomainID: mpa.ID,
			Name:         "大屏范围测试 MPA 完整",
			Grade:        mpaGrade,
			Mobile:       &mobile,
			WorkUnit:     &workUnit,
			Mentor:       &mentor,
			Status:       common.AlumniStatusActive,
		}
		mpaIncomplete := &model.AlumniProfile{
			DataDomainID: mpa.ID,
			Name:         "大屏范围测试 MPA 未完整",
			Grade:        mpaGrade,
			Status:       common.AlumniStatusActive,
		}
		academicProfile := &model.AlumniProfile{
			DataDomainID: academic.ID,
			Name:         "大屏范围测试学硕",
			Grade:        academicGrade,
			Status:       common.AlumniStatusActive,
		}
		for _, profile := range []*model.AlumniProfile{mpaComplete, mpaIncomplete, academicProfile} {
			if err := tx.Create(profile).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(&model.User{
			Account:      fmt.Sprintf("dashboard_scope_%d", suffix),
			PasswordHash: "test-password-hash",
			Role:         common.RoleAlumni,
			AlumniID:     &academicProfile.ID,
			Status:       common.UserStatusActive,
		}).Error; err != nil {
			return err
		}

		afterMPA, err := repo.Overview(ctx, []uint64{mpa.ID})
		if err != nil {
			return err
		}
		afterAcademic, err := repo.Overview(ctx, []uint64{academic.ID})
		if err != nil {
			return err
		}
		afterBoth, err := repo.Overview(ctx, []uint64{mpa.ID, academic.ID, mpa.ID})
		if err != nil {
			return err
		}

		assertDashboardStatsDelta(t, beforeMPA, afterMPA, do.DashboardOverviewStats{
			TotalAlumni: 2, MobileComplete: 1, WorkUnitComplete: 1, MentorComplete: 1,
		})
		assertDashboardStatsDelta(t, beforeAcademic, afterAcademic, do.DashboardOverviewStats{
			TotalAlumni: 1, TotalAccounts: 1,
		})
		assertDashboardStatsDelta(t, beforeBoth, afterBoth, do.DashboardOverviewStats{
			TotalAlumni: 3, TotalAccounts: 1, MobileComplete: 1, WorkUnitComplete: 1, MentorComplete: 1,
		})

		items, err := repo.Distribution(ctx, do.DashboardDistributionQuery{
			Dimension:     do.DashboardDistributionDimensionGrade,
			DataDomainIDs: []uint64{mpa.ID},
		})
		if err != nil {
			return err
		}
		if dashboardDistributionValue(items, mpaGrade) < 2 {
			t.Errorf("MPA distribution does not include both MPA profiles: %+v", items)
		}
		if dashboardDistributionValue(items, academicGrade) != 0 {
			t.Errorf("MPA distribution leaked academic profile: %+v", items)
		}

		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("transaction error = %v, want rollback sentinel", err)
	}
}

func assertDashboardStatsDelta(t *testing.T, before, after, want do.DashboardOverviewStats) {
	t.Helper()
	got := do.DashboardOverviewStats{
		TotalAlumni:      after.TotalAlumni - before.TotalAlumni,
		TotalAccounts:    after.TotalAccounts - before.TotalAccounts,
		MobileComplete:   after.MobileComplete - before.MobileComplete,
		WorkUnitComplete: after.WorkUnitComplete - before.WorkUnitComplete,
		MentorComplete:   after.MentorComplete - before.MentorComplete,
	}
	if got != want {
		t.Errorf("dashboard statistics delta = %+v, want %+v", got, want)
	}
}

func dashboardDistributionValue(items []do.DashboardDistributionItem, name string) int64 {
	for _, item := range items {
		if item.Name == name {
			return item.Value
		}
	}
	return 0
}
