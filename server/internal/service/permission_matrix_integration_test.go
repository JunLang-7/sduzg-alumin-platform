package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/repository"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// TestPermissionMatrixAcrossBusinessEntrypoints 使用真实 MySQL 验证管理员授权的关键入口。
// 测试通过事务回滚，不会保留任何校友档案或审计日志。
func TestPermissionMatrixAcrossBusinessEntrypoints(t *testing.T) {
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

	var undergraduate, academic, mpa model.DataDomain
	for _, target := range []struct {
		code string
		dest *model.DataDomain
	}{
		{common.DataDomainUndergraduate, &undergraduate},
		{common.DataDomainAcademicGraduate, &academic},
		{common.DataDomainMPA, &mpa},
	} {
		if err := db.Where("code = ?", target.code).First(target.dest).Error; err != nil {
			t.Fatalf("find %s data domain: %v", target.code, err)
		}
	}

	rollback := errors.New("rollback permission matrix test transaction")
	err = db.Transaction(func(tx *gorm.DB) error {
		ctx := context.Background()
		tag := fmt.Sprintf("pm-%d", time.Now().UnixNano())
		secretMobile := "13900000000"
		secretWorkUnit := "权限矩阵测试单位"
		profiles := []*model.AlumniProfile{
			{DataDomainID: undergraduate.ID, Name: tag + "-undergraduate", Grade: tag + "-u", Mobile: &secretMobile, WorkUnit: &secretWorkUnit, Status: common.AlumniStatusActive},
			{DataDomainID: academic.ID, Name: tag + "-academic", Grade: tag + "-a", Mobile: &secretMobile, WorkUnit: &secretWorkUnit, Status: common.AlumniStatusActive},
			{DataDomainID: mpa.ID, Name: tag + "-mpa", Grade: tag + "-m", Mobile: &secretMobile, WorkUnit: &secretWorkUnit, Status: common.AlumniStatusActive},
		}
		for _, profile := range profiles {
			if err := tx.Create(profile).Error; err != nil {
				return err
			}
		}

		alumniService := NewAlumniService(repository.NewAlumniRepository(tx), nil).
			WithOperationLogger(NewOperationLogger(tx))
		dashboardService := NewDashboardService(repository.NewDashboardRepository(tx))
		mpaAdmin := common.AccessContext{UserID: 701, Role: common.RoleAdmin, DomainIDs: []uint64{mpa.ID}}
		mpaSensitiveAdmin := common.AccessContext{
			UserID:    702,
			Role:      common.RoleAdmin,
			DomainIDs: []uint64{mpa.ID},
			Permissions: map[string]bool{
				common.PermissionAlumniSensitiveRead: true,
			},
		}
		superAdmin := common.AccessContext{UserID: 703, Role: common.RoleSuperAdmin}

		// 列表、分页总数和敏感字段：无敏感权限的单域管理员只能得到本域的一条脱敏记录。
		list, err := alumniService.List(ctx, dto.AlumniListRequest{Keyword: tag}, mpaAdmin)
		if err != nil {
			return err
		}
		if list.Total != 1 || len(list.Items) != 1 || list.Items[0].ID != profiles[2].ID || list.Items[0].Mobile != nil || list.Items[0].WorkUnit != nil {
			t.Errorf("MPA no-sensitive list = %+v, want one masked MPA profile", list)
		}
		if _, err := alumniService.List(ctx, dto.AlumniListRequest{DataDomainID: &academic.ID}, mpaAdmin); !errors.Is(err, common.ErrPermissionDenied) {
			t.Errorf("requested out-of-domain list error = %v, want permission denied", err)
		}
		allDomains, err := alumniService.List(ctx, dto.AlumniListRequest{Keyword: tag}, superAdmin)
		if err != nil || allDomains.Total != 3 {
			t.Errorf("super-admin list = total %d, err %v; want all three domains", allDomains.Total, err)
		}

		// 直接 ID 枚举与写入：域外对象按不存在处理，且不会被更新。
		if _, err := alumniService.GetByID(ctx, profiles[1].ID, mpaAdmin); !errors.Is(err, common.ErrAlumniNotFound) {
			t.Errorf("cross-domain detail error = %v, want alumni not found", err)
		}
		if _, err := alumniService.Update(ctx, mpaAdmin, profiles[1].ID, dto.AdminAlumniUpdateRequest{Name: "不应写入", Grade: profiles[1].Grade}); !errors.Is(err, common.ErrAlumniNotFound) {
			t.Errorf("cross-domain update error = %v, want alumni not found", err)
		}

		// 敏感字段权限与导出：无权限导出仅含本域，且 CSV 中不存在敏感原值；审计只记录汇总信息。
		sensitiveList, err := alumniService.List(ctx, dto.AlumniListRequest{Keyword: tag}, mpaSensitiveAdmin)
		if err != nil || len(sensitiveList.Items) != 1 || sensitiveList.Items[0].Mobile == nil || sensitiveList.Items[0].WorkUnit == nil {
			t.Errorf("MPA sensitive list = %+v, err %v; want sensitive fields", sensitiveList, err)
		}
		exportResult, err := alumniService.Export(ctx, dto.AlumniExportRequest{Format: "csv", Keyword: tag}, mpaAdmin)
		if err != nil {
			return err
		}
		records, err := csv.NewReader(bytes.NewReader(exportResult.Data[3:])).ReadAll()
		if err != nil {
			return fmt.Errorf("read export csv: %w", err)
		}
		if len(records) != 2 || records[1][0] != profiles[2].Name || strings.Contains(string(exportResult.Data), secretMobile) || strings.Contains(string(exportResult.Data), secretWorkUnit) {
			t.Errorf("masked MPA export leaked data or domains: %q", exportResult.Data)
		}
		var exportLog model.OperationLog
		if err := tx.Where("action = ? AND operator_id = ?", "export_alumni", mpaAdmin.UserID).Order("id DESC").First(&exportLog).Error; err != nil {
			return fmt.Errorf("find export audit: %w", err)
		}
		if exportLog.Detail == nil || strings.Contains(*exportLog.Detail, secretMobile) || strings.Contains(*exportLog.Detail, secretWorkUnit) {
			t.Errorf("export audit contains sensitive value: %+v", exportLog)
		}

		// 大屏统计：本域管理员只能得到 MPA 年级，空 scope 默认拒绝。
		distribution, err := dashboardService.Distribution(ctx, dto.DashboardDistributionRequest{Dimension: "grade"}, mpaAdmin)
		if err != nil {
			return err
		}
		if permissionMatrixDistributionValue(distribution, profiles[2].Grade) != 1 || permissionMatrixDistributionValue(distribution, profiles[0].Grade) != 0 || permissionMatrixDistributionValue(distribution, profiles[1].Grade) != 0 {
			t.Errorf("MPA dashboard distribution leaked another domain: %+v", distribution)
		}
		noScopeAdmin := common.AccessContext{UserID: 704, Role: common.RoleAdmin}
		if _, err := alumniService.List(ctx, dto.AlumniListRequest{Keyword: tag}, noScopeAdmin); !errors.Is(err, common.ErrPermissionDenied) {
			t.Errorf("no-scope list error = %v, want permission denied", err)
		}
		if _, err := dashboardService.Overview(ctx, noScopeAdmin); !errors.Is(err, common.ErrPermissionDenied) {
			t.Errorf("no-scope dashboard error = %v, want permission denied", err)
		}

		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("transaction error = %v, want rollback sentinel", err)
	}
}

func permissionMatrixDistributionValue(items []dto.DashboardDistributionItem, name string) int64 {
	for _, item := range items {
		if item.Name == name {
			return item.Value
		}
	}
	return 0
}
