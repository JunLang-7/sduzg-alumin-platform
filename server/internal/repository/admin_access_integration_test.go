package repository

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/query"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestUserRepositoryAdminAccessTransactionAndAudit(t *testing.T) {
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

	var undergraduate, mpa model.DataDomain
	dataDomainQuery := query.Use(db).DataDomain
	if err := db.Where(dataDomainQuery.Code.Eq(common.DataDomainUndergraduate)).First(&undergraduate).Error; err != nil {
		t.Fatalf("find undergraduate data domain: %v", err)
	}
	if err := db.Where(dataDomainQuery.Code.Eq(common.DataDomainMPA)).First(&mpa).Error; err != nil {
		t.Fatalf("find MPA data domain: %v", err)
	}

	rollback := errors.New("rollback test transaction")
	err = db.Transaction(func(tx *gorm.DB) error {
		repo := NewUserRepository(tx)
		queries := query.Use(tx)
		ctx := context.Background()
		passwordHash := "bcrypt-hash-not-plaintext"
		created, err := repo.CreateAdminWithAccess(ctx, do.AdminCreateProfile{Account: "admin_access_test"}, passwordHash,
			[]uint64{mpa.ID}, []string{common.PermissionAlumniSensitiveRead}, 1)
		if err != nil {
			return err
		}

		var scopes []model.AdminDataScope
		if err := tx.Where(queries.AdminDataScope.UserID.Eq(created.ID)).Find(&scopes).Error; err != nil {
			return err
		}
		if len(scopes) != 1 || scopes[0].DataDomainID != mpa.ID {
			t.Errorf("created scopes = %+v, want MPA %d", scopes, mpa.ID)
		}
		var permissions []model.AdminPermission
		if err := tx.Where(queries.AdminPermission.UserID.Eq(created.ID)).Find(&permissions).Error; err != nil {
			return err
		}
		if len(permissions) != 1 || permissions[0].PermissionCode != common.PermissionAlumniSensitiveRead {
			t.Errorf("created permissions = %+v", permissions)
		}

		updated, err := repo.ReplaceAdminAccess(ctx, created.ID,
			[]uint64{undergraduate.ID, mpa.ID}, []string{common.PermissionAlumniFilesManage}, 1)
		if err != nil {
			return err
		}
		if updated.ID != created.ID {
			t.Errorf("updated id = %d, want %d", updated.ID, created.ID)
		}
		if err := tx.Where(queries.AdminDataScope.UserID.Eq(created.ID)).Order(queries.AdminDataScope.DataDomainID.Asc()).Find(&scopes).Error; err != nil {
			return err
		}
		if len(scopes) != 2 || scopes[0].DataDomainID != undergraduate.ID || scopes[1].DataDomainID != mpa.ID {
			t.Errorf("replaced scopes = %+v", scopes)
		}
		if err := tx.Where(queries.AdminPermission.UserID.Eq(created.ID)).Find(&permissions).Error; err != nil {
			return err
		}
		if len(permissions) != 1 || permissions[0].PermissionCode != common.PermissionAlumniFilesManage {
			t.Errorf("replaced permissions = %+v", permissions)
		}

		var logs []model.OperationLog
		if err := tx.Where(queries.OperationLog.TargetType.Eq("admin_access"), queries.OperationLog.TargetID.Eq(created.ID)).Order(queries.OperationLog.ID.Asc()).Find(&logs).Error; err != nil {
			return err
		}
		if len(logs) != 2 || logs[0].Action != "create_admin_access" || logs[1].Action != "replace_admin_access" {
			t.Errorf("unexpected authorization audit logs: %+v", logs)
		}
		var createAudit, replaceAudit adminAccessAuditDetail
		if logs[0].Detail == nil || json.Unmarshal([]byte(*logs[0].Detail), &createAudit) != nil {
			t.Errorf("create audit detail is invalid: %+v", logs[0].Detail)
		} else if createAudit.Before != nil || len(createAudit.After.DomainIDs) != 1 || createAudit.After.DomainIDs[0] != mpa.ID || len(createAudit.After.Permissions) != 1 || createAudit.After.Permissions[0] != common.PermissionAlumniSensitiveRead {
			t.Errorf("unexpected create audit snapshot: %+v", createAudit)
		}
		if logs[1].Detail == nil || json.Unmarshal([]byte(*logs[1].Detail), &replaceAudit) != nil {
			t.Errorf("replace audit detail is invalid: %+v", logs[1].Detail)
		} else if replaceAudit.Before == nil || len(replaceAudit.Before.DomainIDs) != 1 || replaceAudit.Before.DomainIDs[0] != mpa.ID || len(replaceAudit.Before.Permissions) != 1 || replaceAudit.Before.Permissions[0] != common.PermissionAlumniSensitiveRead || len(replaceAudit.After.DomainIDs) != 2 || replaceAudit.After.DomainIDs[0] != undergraduate.ID || replaceAudit.After.DomainIDs[1] != mpa.ID || len(replaceAudit.After.Permissions) != 1 || replaceAudit.After.Permissions[0] != common.PermissionAlumniFilesManage {
			t.Errorf("unexpected replace audit snapshot: %+v", replaceAudit)
		}
		for _, log := range logs {
			if log.Detail != nil && strings.Contains(*log.Detail, passwordHash) {
				t.Errorf("audit log contains password hash: %s", *log.Detail)
			}
		}

		_, err = repo.CreateAdminWithAccess(ctx, do.AdminCreateProfile{Account: "admin_access_rollback_test"}, passwordHash,
			[]uint64{mpa.ID}, []string{"duplicate", "duplicate"}, 1)
		if err == nil {
			t.Error("expected duplicate permission write to fail")
		}
		var rolledBackCount int64
		if err := tx.Model(&model.User{}).Where(queries.User.Account.Eq("admin_access_rollback_test")).Count(&rolledBackCount).Error; err != nil {
			return err
		}
		if rolledBackCount != 0 {
			t.Errorf("failed transaction left %d user records", rolledBackCount)
		}

		if err := tx.Model(&model.DataDomain{}).Where(queries.DataDomain.ID.Eq(mpa.ID)).Update(queries.DataDomain.Status.ColumnName().String(), common.DataDomainStatusDisabled).Error; err != nil {
			return err
		}
		_, err = repo.CreateAdminWithAccess(ctx, do.AdminCreateProfile{Account: "admin_access_disabled_domain_test"}, passwordHash,
			[]uint64{mpa.ID}, nil, 1)
		if !errors.Is(err, common.ErrInvalidDataDomain) {
			t.Errorf("disabled data domain error = %v, want %v", err, common.ErrInvalidDataDomain)
		}

		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("transaction error = %v, want rollback sentinel", err)
	}
}
