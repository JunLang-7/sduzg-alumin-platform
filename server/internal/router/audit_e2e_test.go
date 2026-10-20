package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestAuditOperationHistoryE2E(t *testing.T) {
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

	var operatorID, viewerID, profileID uint64
	t.Cleanup(func() {
		if profileID != 0 {
			_ = db.Where("id = ?", profileID).Delete(&model.AlumniProfile{}).Error
		}
		if operatorID != 0 {
			_ = db.Where("operator_id = ?", operatorID).Delete(&model.OperationLog{}).Error
		}
		if viewerID != 0 {
			_ = db.Where("user_id = ?", viewerID).Delete(&model.AdminDataScope{}).Error
			_ = db.Where("user_id = ?", viewerID).Delete(&model.AdminPermission{}).Error
		}
		if operatorID != 0 {
			_ = db.Where("id = ?", operatorID).Delete(&model.User{}).Error
		}
		if viewerID != 0 {
			_ = db.Where("id = ?", viewerID).Delete(&model.User{}).Error
		}
	})

	err = func(tx *gorm.DB) error {
		var domain model.DataDomain
		if err := tx.Where("code = ? AND status = ?", common.DataDomainMPA, common.DataDomainStatusActive).First(&domain).Error; err != nil {
			return fmt.Errorf("find MPA data domain: %w", err)
		}

		suffix := time.Now().UnixNano()
		operator := &model.User{
			Account:      fmt.Sprintf("audit-e2e-%d", suffix),
			PasswordHash: "not-used-by-e2e",
			Role:         common.RoleSuperAdmin,
			RealName:     ptrString("E2E超级管理员"),
			Status:       common.UserStatusActive,
		}
		if err := tx.Create(operator).Error; err != nil {
			return fmt.Errorf("create e2e operator: %w", err)
		}
		operatorID = operator.ID
		viewer := &model.User{
			Account:      fmt.Sprintf("audit-e2e-viewer-%d", suffix),
			PasswordHash: "not-used-by-e2e",
			Role:         common.RoleAdmin,
			RealName:     ptrString("E2E领域管理员"),
			Status:       common.UserStatusActive,
		}
		if err := tx.Create(viewer).Error; err != nil {
			return fmt.Errorf("create e2e viewer: %w", err)
		}
		viewerID = viewer.ID
		if err := tx.Create(&model.AdminDataScope{UserID: viewer.ID, DataDomainID: domain.ID}).Error; err != nil {
			return fmt.Errorf("create e2e viewer data scope: %w", err)
		}

		profile := &model.AlumniProfile{
			DataDomainID: domain.ID,
			Name:         fmt.Sprintf("操作历史E2E校友-%d", suffix),
			Grade:        "2020级",
			WorkUnit:     ptrString("旧单位"),
			Mobile:       ptrString("13800000000"),
			Status:       common.AlumniStatusActive,
			CreatedBy:    &operator.ID,
			UpdatedBy:    &operator.ID,
		}
		if err := tx.Create(profile).Error; err != nil {
			return fmt.Errorf("create e2e alumni: %w", err)
		}
		profileID = profile.ID

		gin.SetMode(gin.TestMode)
		secret := "audit-e2e-secret"
		engine := New(Dependencies{
			Config: config.Config{
				App:  config.AppConfig{Name: "audit-e2e", Env: config.EnvDevelopment},
				Auth: config.AuthConfig{JWTSecret: secret, AccessTokenTTL: time.Hour},
			},
			Logger: zap.NewNop(),
			DB:     tx,
		})
		token := testAccessTokenForUser(t, secret, operator.ID, time.Now().Add(time.Hour))

		updateRequest := httptest.NewRequest(
			http.MethodPut,
			fmt.Sprintf("/api/v1/admin/alumni/%d", profile.ID),
			strings.NewReader(fmt.Sprintf(`{"name":%q,"grade":"2021级","work_unit":"新单位","mobile":"13900000000"}`, profile.Name)),
		)
		updateRequest.Header.Set("Authorization", "Bearer "+token)
		updateRequest.Header.Set("Content-Type", "application/json")
		updateResponse := httptest.NewRecorder()
		engine.ServeHTTP(updateResponse, updateRequest)
		if updateResponse.Code != http.StatusOK {
			return fmt.Errorf("update status = %d, body = %s", updateResponse.Code, updateResponse.Body.String())
		}

		var log model.OperationLog
		if err := tx.Where("operator_id = ? AND target_type = ? AND target_id = ? AND action = ?", operator.ID, "alumni_profile", profile.ID, "update").
			Order("id DESC").
			First(&log).Error; err != nil {
			return fmt.Errorf("find operation log written by update flow: %w", err)
		}

		query := url.Values{}
		query.Set("action", "update")
		query.Set("management_scope", "MPA专业学位研究生")
		query.Set("target_id", fmt.Sprintf("%d", profile.ID))
		listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit/operations?"+query.Encode(), nil)
		listRequest.Header.Set("Authorization", "Bearer "+token)
		listResponse := httptest.NewRecorder()
		engine.ServeHTTP(listResponse, listRequest)
		if listResponse.Code != http.StatusOK {
			return fmt.Errorf("list status = %d, body = %s", listResponse.Code, listResponse.Body.String())
		}

		var listBody struct {
			Code int `json:"code"`
			Data struct {
				Items []struct {
					Operator   string
					ID         uint64 `json:"id"`
					TargetName string `json:"target_name"`
					Action     string `json:"action"`
				} `json:"items"`
				Total int64 `json:"total"`
			} `json:"data"`
		}
		if err := json.Unmarshal(listResponse.Body.Bytes(), &listBody); err != nil {
			return fmt.Errorf("decode list response: %w", err)
		}
		if listBody.Code != 0 || listBody.Data.Total != 1 || len(listBody.Data.Items) != 1 {
			return fmt.Errorf("unexpected list response: %s", listResponse.Body.String())
		}
		if listBody.Data.Items[0].ID != log.ID || listBody.Data.Items[0].Operator != "E2E超级管理员" || listBody.Data.Items[0].TargetName != profile.Name || listBody.Data.Items[0].Action != "update" {
			return fmt.Errorf("unexpected list item: %+v", listBody.Data.Items[0])
		}

		viewerToken := testAccessTokenForUser(t, secret, viewer.ID, time.Now().Add(time.Hour))
		viewerRequest := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit/operations?"+query.Encode(), nil)
		viewerRequest.Header.Set("Authorization", "Bearer "+viewerToken)
		viewerResponse := httptest.NewRecorder()
		engine.ServeHTTP(viewerResponse, viewerRequest)
		if viewerResponse.Code != http.StatusOK || !strings.Contains(viewerResponse.Body.String(), `"operator":"E2E超级管理员"`) {
			return fmt.Errorf("viewer could not see in-domain super-admin operation: status=%d body=%s", viewerResponse.Code, viewerResponse.Body.String())
		}

		detailRequest := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/admin/audit/operations/%d", log.ID), nil)
		detailRequest.Header.Set("Authorization", "Bearer "+token)
		detailResponse := httptest.NewRecorder()
		engine.ServeHTTP(detailResponse, detailRequest)
		if detailResponse.Code != http.StatusOK {
			return fmt.Errorf("detail status = %d, body = %s", detailResponse.Code, detailResponse.Body.String())
		}
		bodyText := detailResponse.Body.String()
		if strings.Contains(bodyText, "13800000000") || strings.Contains(bodyText, "13900000000") {
			return fmt.Errorf("sensitive value leaked in detail response: %s", bodyText)
		}
		var detailBody struct {
			Code int `json:"code"`
			Data struct {
				Changes []struct {
					FieldName string `json:"field_name"`
					OldValue  string `json:"old_value"`
					NewValue  string `json:"new_value"`
				} `json:"changes"`
			} `json:"data"`
		}
		if err := json.Unmarshal(detailResponse.Body.Bytes(), &detailBody); err != nil {
			return fmt.Errorf("decode detail response: %w", err)
		}
		if detailBody.Code != 0 || len(detailBody.Data.Changes) != 3 {
			return fmt.Errorf("unexpected detail response: %s", bodyText)
		}
		changesByField := make(map[string]struct {
			oldValue string
			newValue string
		}, len(detailBody.Data.Changes))
		for _, change := range detailBody.Data.Changes {
			changesByField[change.FieldName] = struct {
				oldValue string
				newValue string
			}{oldValue: change.OldValue, newValue: change.NewValue}
		}
		if change, ok := changesByField["grade"]; !ok || change.oldValue != "2020级" || change.newValue != "2021级" {
			return fmt.Errorf("unexpected public diff: %+v", changesByField["grade"])
		}
		for _, field := range []string{"work_unit", "mobile"} {
			change, ok := changesByField[field]
			if !ok || change.oldValue != "已填写" || change.newValue != "已修改" {
				return fmt.Errorf("unexpected sensitive diff for %s: %+v", field, change)
			}
		}

		return nil
	}(db)
	if err != nil {
		t.Fatalf("e2e error = %v", err)
	}
}

func TestAuditOperationListWithOversizedDetail(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN is not configured")
	}

	// 固定会话 sort_buffer_size 为 MySQL 默认值 256KB，复现线上默认配置，
	// 避免本机把该值调大后回归用例失效。go-sql-driver 会把未知参数作为会话变量下发。
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	db, err := gorm.Open(mysql.Open(dsn+separator+"sort_buffer_size=262144"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get test database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	suffix := time.Now().UnixNano()
	operator := &model.User{
		Account:      fmt.Sprintf("audit-oversized-%d", suffix),
		PasswordHash: "not-used-by-e2e",
		Role:         common.RoleSuperAdmin,
		RealName:     ptrString("超大明细管理员"),
		Status:       common.UserStatusActive,
	}
	if err := db.Create(operator).Error; err != nil {
		t.Fatalf("create e2e operator: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Where("operator_id = ?", operator.ID).Delete(&model.OperationLog{}).Error
		_ = db.Where("id = ?", operator.ID).Delete(&model.User{}).Error
	})

	// 先写入若干小记录，保证 MySQL 走 filesort，而不是对单行跳过排序。
	for i := 0; i < 20; i++ {
		small := fmt.Sprintf(`{"schema_version":1,"operator_name":"普通记录-%d","target_name":"校友档案批量导入"}`, i)
		if err := db.Create(&model.OperationLog{
			OperatorID:   operator.ID,
			OperatorRole: common.RoleSuperAdmin,
			Action:       "update",
			TargetType:   "alumni_profile",
			Detail:       &small,
		}).Error; err != nil {
			t.Fatalf("create small operation log: %v", err)
		}
	}

	const recordCount = 600
	bigDetail := buildOversizedAuditDetail(t, recordCount, 3000)
	if len(bigDetail) < 1024*1024 {
		t.Fatalf("oversized detail is too small: %d bytes", len(bigDetail))
	}
	bigLog := &model.OperationLog{
		OperatorID:   operator.ID,
		OperatorRole: common.RoleSuperAdmin,
		Action:       "import",
		TargetType:   "alumni_batch",
		Detail:       &bigDetail,
	}
	if err := db.Create(bigLog).Error; err != nil {
		t.Fatalf("create oversized operation log: %v", err)
	}

	gin.SetMode(gin.TestMode)
	secret := "audit-oversized-secret"
	engine := New(Dependencies{
		Config: config.Config{
			App:  config.AppConfig{Name: "audit-oversized-e2e", Env: config.EnvDevelopment},
			Auth: config.AuthConfig{JWTSecret: secret, AccessTokenTTL: time.Hour},
		},
		Logger: zap.NewNop(),
		DB:     db,
	})
	token := testAccessTokenForUser(t, secret, operator.ID, time.Now().Add(time.Hour))

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit/operations?page=1&page_size=50", nil)
	listRequest.Header.Set("Authorization", "Bearer "+token)
	listResponse := httptest.NewRecorder()
	engine.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listResponse.Code, listResponse.Body.String())
	}

	var listBody struct {
		Code int `json:"code"`
		Data struct {
			Items []struct {
				ID              uint64 `json:"id"`
				Operator        string `json:"operator"`
				TargetName      string `json:"target_name"`
				ManagementScope string `json:"management_scope"`
				Source          string `json:"source"`
				Reason          string `json:"reason"`
				Status          string `json:"status"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listResponse.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if listBody.Code != 0 {
		t.Fatalf("list code = %d, body = %s", listBody.Code, listResponse.Body.String())
	}
	var found bool
	for _, item := range listBody.Data.Items {
		if item.ID != bigLog.ID {
			continue
		}
		found = true
		if item.Operator != "超大明细管理员" || item.TargetName != "校友档案批量导入" || item.ManagementScope != "MPA专业学位研究生" || item.Source != "admin_import" || item.Reason != "导入校友档案" || item.Status != "applied" {
			t.Fatalf("unexpected oversized list item: %+v", item)
		}
	}
	if !found {
		t.Fatalf("oversized operation log %d not found in list: %+v", bigLog.ID, listBody.Data.Items)
	}

	detailRequest := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/admin/audit/operations/%d", bigLog.ID), nil)
	detailRequest.Header.Set("Authorization", "Bearer "+token)
	detailResponse := httptest.NewRecorder()
	engine.ServeHTTP(detailResponse, detailRequest)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("detail status = %d, body = %s", detailResponse.Code, detailResponse.Body.String())
	}
	var detailBody struct {
		Code int `json:"code"`
		Data struct {
			BatchCreatedAlumni          []json.RawMessage `json:"batch_created_alumni"`
			BatchCreatedAlumniTotal     int               `json:"batch_created_alumni_total"`
			BatchCreatedAlumniTruncated bool              `json:"batch_created_alumni_truncated"`
		} `json:"data"`
	}
	if err := json.Unmarshal(detailResponse.Body.Bytes(), &detailBody); err != nil {
		t.Fatalf("decode detail response: %v", err)
	}
	if detailBody.Code != 0 || len(detailBody.Data.BatchCreatedAlumni) != recordCount {
		t.Fatalf("unexpected detail payload: code=%d records=%d", detailBody.Code, len(detailBody.Data.BatchCreatedAlumni))
	}
	if detailBody.Data.BatchCreatedAlumniTotal != recordCount || !detailBody.Data.BatchCreatedAlumniTruncated {
		t.Fatalf("unexpected detail truncation metadata: total=%d truncated=%v", detailBody.Data.BatchCreatedAlumniTotal, detailBody.Data.BatchCreatedAlumniTruncated)
	}
}

func buildOversizedAuditDetail(t *testing.T, recordCount, payloadSize int) string {
	t.Helper()
	payload := strings.Repeat("x", payloadSize)
	records := make([]map[string]any, 0, recordCount)
	for i := 0; i < recordCount; i++ {
		name := fmt.Sprintf("批量校友%04d", i+1)
		records = append(records, map[string]any{
			"id":   i + 1,
			"name": name,
			"field_values": map[string]string{
				"name":  name,
				"grade": payload,
			},
		})
	}
	detail, err := json.Marshal(map[string]any{
		"schema_version":                 1,
		"operator_name":                  "超大明细管理员",
		"operator_role_label":            "超级管理员",
		"target_name":                    "校友档案批量导入",
		"target_meta":                    fmt.Sprintf("%d 条校友档案", recordCount),
		"management_scope":               "MPA专业学位研究生",
		"source":                         "admin_import",
		"reason":                         "导入校友档案",
		"status":                         "applied",
		"batch_created_alumni":           records,
		"batch_created_alumni_total":     recordCount,
		"batch_created_alumni_truncated": true,
	})
	if err != nil {
		t.Fatalf("marshal oversized detail: %v", err)
	}
	return string(detail)
}

func ptrString(value string) *string {
	return &value
}
