package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/response"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// 使用真实路由、JWT 和授权加载器，避免仅验证独立中间件而漏掉路由注册。
func TestAlumniQueryAccessIntegration(t *testing.T) {
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
		t.Fatalf("get test database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	// 授权加载器并行读取数据域和权限，不能共用单连接事务。
	// 使用独立测试库与显式清理，让请求走真实连接池。
	var userIDs, profileIDs []uint64
	t.Cleanup(func() {
		for _, cleanup := range []struct {
			model any
			field string
			ids   []uint64
		}{
			{&model.HistoryContribution{}, "author_user_id", userIDs},
			{&model.OperationLog{}, "operator_id", userIDs},
			{&model.AdminDataScope{}, "user_id", userIDs},
			{&model.AdminPermission{}, "user_id", userIDs},
			{&model.User{}, "id", userIDs},
			{&model.AlumniProfile{}, "id", profileIDs},
		} {
			if len(cleanup.ids) > 0 {
				if err := db.Unscoped().Where(cleanup.field+" IN ?", cleanup.ids).Delete(cleanup.model).Error; err != nil {
					t.Errorf("clean up fixture %T: %v", cleanup.model, err)
				}
			}
		}
	})

	var localDomain, foreignDomain model.DataDomain
	for _, domain := range []struct {
		code string
		dest *model.DataDomain
	}{
		{common.DataDomainMPA, &localDomain},
		{common.DataDomainUndergraduate, &foreignDomain},
	} {
		if err := db.Where("code = ? AND status = ?", domain.code, common.DataDomainStatusActive).First(domain.dest).Error; err != nil {
			t.Fatalf("find domain %s: %v", domain.code, err)
		}
	}
	tag := fmt.Sprintf("query-access-%d", time.Now().UnixNano())
	profiles := []*model.AlumniProfile{
		{DataDomainID: localDomain.ID, Name: tag + "-self", Grade: "2020", Mobile: ptrString("13800000001"), WorkUnit: ptrString("测试单位"), Status: common.AlumniStatusActive},
		{DataDomainID: localDomain.ID, Name: tag + "-other", Grade: "2020", Mobile: ptrString("13800000002"), Status: common.AlumniStatusActive},
		{DataDomainID: foreignDomain.ID, Name: tag + "-foreign", Grade: "2020", Mobile: ptrString("13800000003"), Status: common.AlumniStatusActive},
	}
	for _, profile := range profiles {
		if err := db.Create(profile).Error; err != nil {
			t.Fatalf("create profile fixture: %v", err)
		}
		profileIDs = append(profileIDs, profile.ID)
	}
	users := map[string]*model.User{
		"alumni":          {Role: common.RoleAlumni, AlumniID: &profiles[0].ID},
		"unbound-alumni":  {Role: common.RoleAlumni},
		"admin":           {Role: common.RoleAdmin},
		"sensitive-admin": {Role: common.RoleAdmin},
		"unscoped-admin":  {Role: common.RoleAdmin},
		"super-admin":     {Role: common.RoleSuperAdmin},
	}
	for label, user := range users {
		user.Account = tag + "-" + label
		user.PasswordHash = "not-used-by-query-test"
		user.Status = common.UserStatusActive
		if err := db.Create(user).Error; err != nil {
			t.Fatalf("create user fixture: %v", err)
		}
		userIDs = append(userIDs, user.ID)
		if label == "admin" || label == "sensitive-admin" {
			if err := db.Create(&model.AdminDataScope{UserID: user.ID, DataDomainID: localDomain.ID}).Error; err != nil {
				t.Fatalf("grant data scope: %v", err)
			}
		}
	}
	if err := db.Create(&model.AdminPermission{UserID: users["sensitive-admin"].ID, PermissionCode: common.PermissionAlumniSensitiveRead}).Error; err != nil {
		t.Fatalf("grant sensitive permission: %v", err)
	}

	gin.SetMode(gin.TestMode)
	secret := "query-access-test-secret"
	engine := New(Dependencies{
		Config: config.Config{
			App:  config.AppConfig{Name: "query-access-test", Env: config.EnvDevelopment},
			Auth: config.AuthConfig{JWTSecret: secret, AccessTokenTTL: time.Hour},
		},
		Logger: zap.NewNop(),
		DB:     db,
	})
	tokens := map[string]string{"guest": ""}
	for label, user := range users {
		tokens[label] = testAccessTokenForUser(t, secret, user.ID, time.Now().Add(time.Hour))
	}
	listPath := "/api/v1/alumni?keyword=" + tag
	detailPath := func(profile *model.AlumniProfile) string { return fmt.Sprintf("/api/v1/alumni/%d", profile.ID) }

	for _, label := range []string{"guest", "alumni", "unbound-alumni"} {
		for _, path := range []string{listPath, detailPath(profiles[0]), detailPath(profiles[1]), detailPath(profiles[2]), "/api/v1/alumni/0", "/api/v1/alumni/not-a-number"} {
			t.Run(label+path, func(t *testing.T) {
				wantStatus, wantCode := http.StatusForbidden, response.CodeForbidden
				if label == "guest" {
					wantStatus, wantCode = http.StatusUnauthorized, response.CodeUnauthorized
				}
				rec := performAPIRequest(t, engine, http.MethodGet, path, "", tokens[label])
				var body struct {
					Code int             `json:"code"`
					Data json.RawMessage `json:"data"`
				}
				decodeAPIResponse(t, rec, &body)
				if rec.Code != wantStatus || body.Code != wantCode || len(body.Data) != 0 {
					t.Fatalf("want status=%d code=%d without data, got %d %s", wantStatus, wantCode, rec.Code, rec.Body.String())
				}
			})
		}
	}

	for _, label := range []string{"admin", "sensitive-admin", "super-admin"} {
		t.Run(label+" list and detail", func(t *testing.T) {
			rec := performAPIRequest(t, engine, http.MethodGet, listPath, "", tokens[label])
			var body struct {
				Code int                              `json:"code"`
				Data common.Pager[dto.AlumniListItem] `json:"data"`
			}
			decodeAPIResponse(t, rec, &body)
			wantCount := 2
			if label == "super-admin" {
				wantCount = 3
			}
			if rec.Code != http.StatusOK || body.Code != 0 || body.Data.Total != int64(wantCount) || len(body.Data.Items) != wantCount {
				t.Fatalf("unexpected scoped list: %d %s", rec.Code, rec.Body.String())
			}
			for _, item := range body.Data.Items {
				if label != "super-admin" && item.DataDomainID != localDomain.ID {
					t.Fatalf("list leaked foreign domain: %+v", item)
				}
				if (item.Mobile == nil) != (label == "admin") {
					t.Fatalf("unexpected sensitive field visibility: %+v", item)
				}
			}
			rec = performAPIRequest(t, engine, http.MethodGet, detailPath(profiles[0]), "", tokens[label])
			var detail struct {
				Code int              `json:"code"`
				Data dto.AlumniDetail `json:"data"`
			}
			decodeAPIResponse(t, rec, &detail)
			if rec.Code != http.StatusOK || detail.Code != 0 || detail.Data.ID != profiles[0].ID || (detail.Data.Mobile == nil) != (label == "admin") {
				t.Fatalf("unexpected scoped detail: %d %s", rec.Code, rec.Body.String())
			}
		})
	}

	t.Run("admin domain boundaries", func(t *testing.T) {
		assertAPIStatus(t, engine, http.MethodGet, detailPath(profiles[2]), "", tokens["admin"], http.StatusNotFound)
		assertAPIStatus(t, engine, http.MethodGet, fmt.Sprintf("%s&data_domain_id=%d", listPath, foreignDomain.ID), "", tokens["admin"], http.StatusForbidden)
		assertAPIStatus(t, engine, http.MethodGet, listPath+"&mobile=13800000001", "", tokens["admin"], http.StatusForbidden)
		assertAPIStatus(t, engine, http.MethodGet, listPath, "", tokens["unscoped-admin"], http.StatusForbidden)
		assertAPIStatus(t, engine, http.MethodGet, detailPath(profiles[2]), "", tokens["super-admin"], http.StatusOK)
	})

	t.Run("alumni self-service and history remain accessible", func(t *testing.T) {
		rec := performAPIRequest(t, engine, http.MethodGet, "/api/v1/alumni/me", "", tokens["alumni"])
		var body struct {
			Data dto.AlumniDetail `json:"data"`
		}
		decodeAPIResponse(t, rec, &body)
		if rec.Code != http.StatusOK || body.Data.ID != profiles[0].ID || body.Data.Mobile == nil || *body.Data.Mobile != *profiles[0].Mobile {
			t.Fatalf("self profile unavailable: %d %s", rec.Code, rec.Body.String())
		}
		rec = performAPIRequest(t, engine, http.MethodPut, "/api/v1/alumni/me", `{"work_unit":"更新后的单位"}`, tokens["alumni"])
		decodeAPIResponse(t, rec, &body)
		if rec.Code != http.StatusOK || body.Data.WorkUnit == nil || *body.Data.WorkUnit != "更新后的单位" {
			t.Fatalf("self update unavailable: %d %s", rec.Code, rec.Body.String())
		}
		assertAPIStatus(t, engine, http.MethodPut, "/api/v1/alumni/me/contact", `{}`, tokens["alumni"], http.StatusBadRequest)
		assertAPIStatus(t, engine, http.MethodGet, "/api/v1/history/entries", "", tokens["alumni"], http.StatusOK)
		assertAPIStatus(t, engine, http.MethodGet, "/api/v1/history/contributions/me", "", tokens["alumni"], http.StatusOK)
		assertAPIStatus(t, engine, http.MethodPost, "/api/v1/history/contributions", `{"title":"权限测试投稿","content":"本人仍可投稿","source_note":"测试来源"}`, tokens["alumni"], http.StatusCreated)
		assertAPIStatus(t, engine, http.MethodGet, "/api/v1/alumni/me", "", tokens["admin"], http.StatusForbidden)
	})

	t.Run("legacy token role cannot override database role", func(t *testing.T) {
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"uid":  users["alumni"].ID,
			"role": common.RoleSuperAdmin,
			"exp":  time.Now().Add(time.Hour).Unix(),
		}).SignedString([]byte(secret))
		if err != nil {
			t.Fatal(err)
		}
		assertAPIStatus(t, engine, http.MethodGet, "/api/v1/admin/dashboard/overview?role=super_admin", "", token, http.StatusForbidden)
		assertAPIStatus(t, engine, http.MethodGet, "/api/v1/super-admin/admins", "", token, http.StatusForbidden)
		assertAPIStatus(t, engine, http.MethodPost, "/api/v1/admin/alumni", `{"role":"super_admin"}`, token, http.StatusForbidden)
	})

	t.Run("database role changes affect the same token immediately", func(t *testing.T) {
		user := users["super-admin"]
		originalRole := user.Role
		t.Cleanup(func() {
			if err := db.Model(&model.User{}).Where("id = ?", user.ID).Update("role", originalRole).Error; err != nil {
				t.Errorf("restore original database role: %v", err)
			}
		})
		for _, role := range []string{common.RoleAlumni, "owner", common.RoleSuperAdmin} {
			if err := db.Model(&model.User{}).Where("id = ?", user.ID).Update("role", role).Error; err != nil {
				t.Fatal(err)
			}
			want := http.StatusForbidden
			if role == common.RoleSuperAdmin {
				want = http.StatusOK
			}
			assertAPIStatus(t, engine, http.MethodGet, "/api/v1/super-admin/admins", "", tokens["super-admin"], want)
			if role == "owner" {
				assertAPIStatus(t, engine, http.MethodGet, "/api/v1/history/entries", "", tokens["super-admin"], http.StatusForbidden)
			}
		}
	})

	t.Run("invalid role login returns readable denial without token", func(t *testing.T) {
		user := users["super-admin"]
		originalRole, originalHash := user.Role, user.PasswordHash
		t.Cleanup(func() {
			if err := db.Model(&model.User{}).Where("id = ?", user.ID).
				Updates(map[string]any{"role": originalRole, "password_hash": originalHash}).Error; err != nil {
				t.Errorf("restore login fixture: %v", err)
			}
		})
		hash, err := bcrypt.GenerateFromPassword([]byte("RoleCheck123"), bcrypt.MinCost)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Model(&model.User{}).Where("id = ?", user.ID).Update("password_hash", string(hash)).Error; err != nil {
			t.Fatal(err)
		}
		for _, role := range []string{"owner", "Admin", ""} {
			if err := db.Model(&model.User{}).Where("id = ?", user.ID).Update("role", role).Error; err != nil {
				t.Fatal(err)
			}
			request, err := json.Marshal(map[string]string{"account": user.Account, "password": "RoleCheck123"})
			if err != nil {
				t.Fatal(err)
			}
			rec := performAPIRequest(t, engine, http.MethodPost, "/api/v1/auth/login", string(request), "")
			var body struct {
				Code    int             `json:"code"`
				Message string          `json:"message"`
				Data    json.RawMessage `json:"data"`
			}
			decodeAPIResponse(t, rec, &body)
			if rec.Code != http.StatusForbidden || body.Code != response.CodeForbidden || body.Message != "账号角色无效，请联系管理员" || len(body.Data) != 0 {
				t.Fatalf("invalid role must not receive a login token: %d %s", rec.Code, rec.Body.String())
			}
		}
	})
}
