package router

import (
	"encoding/json"
	"errors"
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

	rollback := errors.New("rollback audit e2e fixture")
	err = db.Transaction(func(tx *gorm.DB) error {
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

		detail, err := json.Marshal(map[string]any{
			"schema_version":      1,
			"operator_name":       "E2E超级管理员",
			"operator_role_label": "超级管理员",
			"target_id":           profile.ID,
			"target_name":         profile.Name,
			"target_meta":         "2020级",
			"management_scope":    "MPA专业学位研究生",
			"source":              "admin",
			"status":              "applied",
			"changes": []map[string]any{
				{"field_name": "work_unit", "field_label": "工作单位", "old_value": "旧单位", "new_value": "新单位", "current_value": "新单位"},
				{"field_name": "mobile", "field_label": "手机号", "old_value": "已填写", "new_value": "已修改", "current_value": "已修改", "sensitive": true},
			},
		})
		if err != nil {
			return fmt.Errorf("marshal e2e audit detail: %w", err)
		}
		detailText := string(detail)
		log := &model.OperationLog{
			OperatorID:   operator.ID,
			OperatorRole: common.RoleSuperAdmin,
			Action:       "update",
			TargetType:   "alumni_profile",
			TargetID:     &profile.ID,
			Detail:       &detailText,
			CreatedAt:    time.Now().Add(-time.Minute),
		}
		if err := tx.Create(log).Error; err != nil {
			return fmt.Errorf("create e2e operation log: %w", err)
		}

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
		token := testAccessToken(t, secret, time.Now().Add(time.Hour))

		query := url.Values{}
		query.Set("action", "update")
		query.Set("management_scope", "MPA专业学位研究生")
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
		if listBody.Data.Items[0].ID != log.ID || listBody.Data.Items[0].TargetName != profile.Name || listBody.Data.Items[0].Action != "update" {
			return fmt.Errorf("unexpected list item: %+v", listBody.Data.Items[0])
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
		if detailBody.Code != 0 || len(detailBody.Data.Changes) != 2 {
			return fmt.Errorf("unexpected detail response: %s", bodyText)
		}
		if detailBody.Data.Changes[0].FieldName != "work_unit" || detailBody.Data.Changes[0].OldValue != "旧单位" {
			return fmt.Errorf("unexpected non-sensitive diff: %+v", detailBody.Data.Changes[0])
		}
		if detailBody.Data.Changes[1].FieldName != "mobile" || detailBody.Data.Changes[1].OldValue != "已填写" || detailBody.Data.Changes[1].NewValue != "已修改" {
			return fmt.Errorf("unexpected sensitive diff: %+v", detailBody.Data.Changes[1])
		}

		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("transaction error = %v, want rollback sentinel", err)
	}
}

func ptrString(value string) *string {
	return &value
}
