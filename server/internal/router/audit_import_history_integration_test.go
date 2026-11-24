package router

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/dto"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestImportedAlumniOperationHistory(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN is not configured")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	var mpa, undergraduate model.DataDomain
	must(db.Where("code = ?", common.DataDomainMPA).First(&mpa).Error)
	must(db.Where("code = ?", common.DataDomainUndergraduate).First(&undergraduate).Error)
	suffix := time.Now().UnixNano()
	var userIDs []uint64
	var importerID uint64
	t.Cleanup(func() {
		if importerID != 0 {
			profileIDs := db.Unscoped().Model(&model.AlumniProfile{}).Select("id").Where("created_by = ?", importerID)
			must(db.Unscoped().Where("alumni_id IN (?)", profileIDs).Delete(&model.AlumniEnrollment{}).Error)
			must(db.Unscoped().Where("created_by = ?", importerID).Delete(&model.AlumniProfile{}).Error)
			must(db.Where("operator_id = ?", importerID).Delete(&model.OperationLog{}).Error)
		}
		if len(userIDs) > 0 {
			must(db.Where("user_id IN ?", userIDs).Delete(&model.AdminDataScope{}).Error)
			must(db.Where("id IN ?", userIDs).Delete(&model.User{}).Error)
		}
	})
	createUser := func(label, role string, domainID uint64) model.User {
		user := model.User{
			Account: fmt.Sprintf("audit-import-%s-%d", label, suffix),
			Role:    role, PasswordHash: "unused", Status: common.UserStatusActive,
		}
		must(db.Create(&user).Error)
		userIDs = append(userIDs, user.ID)
		if domainID != 0 {
			must(db.Create(&model.AdminDataScope{UserID: user.ID, DataDomainID: domainID}).Error)
		}
		return user
	}
	operator := createUser("super", common.RoleSuperAdmin, 0)
	importerID = operator.ID
	mpaAdmin := createUser("mpa", common.RoleAdmin, mpa.ID)
	undergraduateAdmin := createUser("undergraduate", common.RoleAdmin, undergraduate.ID)
	alumniUser := createUser("alumni", common.RoleAlumni, 0)
	const secret = "import-history-integration-secret"
	gin.SetMode(gin.TestMode)
	engine := New(Dependencies{
		DB: db, Logger: zap.NewNop(),
		Config: config.Config{
			App:  config.AppConfig{Env: config.EnvDevelopment},
			Auth: config.AuthConfig{JWTSecret: secret, AccessTokenTTL: time.Hour},
		},
	})
	call := func(method, path string, userID uint64, contentType string, body io.Reader, status int, data any) {
		t.Helper()
		req := httptest.NewRequest(method, path, body)
		if userID != 0 {
			token := testAccessTokenForUser(t, secret, userID, time.Now().Add(time.Hour))
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		res := httptest.NewRecorder()
		engine.ServeHTTP(res, req)
		if res.Code != status {
			t.Fatalf("%s %s: status %d, want %d: %s", method, path, res.Code, status, res.Body)
		}
		if data != nil {
			var envelope struct {
				Code int             `json:"code"`
				Data json.RawMessage `json:"data"`
			}
			must(json.Unmarshal(res.Body.Bytes(), &envelope))
			if envelope.Code != 0 {
				t.Fatalf("%s %s: unexpected code %d", method, path, envelope.Code)
			}
			must(json.Unmarshal(envelope.Data, data))
		}
	}

	var upload bytes.Buffer
	form := multipart.NewWriter(&upload)
	part, err := form.CreateFormFile("file", "audit-import.csv")
	must(err)
	writer := csv.NewWriter(part)
	must(writer.Write([]string{"姓名", "培养层次", "年级", "班级", "届数", "辅导员", "导师", "专业", "培养方式",
		"行业", "工作单位", "职务", "通讯地址", "性别", "手机号", "邮箱", "所属领域"}))
	sameName := fmt.Sprintf("审计导入测试同名-%d", suffix)
	lastName := fmt.Sprintf("审计导入测试末行-%d", suffix)
	row := []string{sameName, "硕士", "2020级", "", "", "", "", "", "", "", "", "", "", "", "", "", common.DataDomainMPA}
	must(writer.Write(row))
	for i := 1; i < 200; i++ {
		row[0], row[2] = fmt.Sprintf("审计导入测试-%d-%d", suffix, i), "2020级"
		if i == 1 {
			row[0], row[2] = sameName, "2021级"
		}
		must(writer.Write(row))
	}
	row[0], row[1], row[16] = lastName, "本科", common.DataDomainUndergraduate
	must(writer.Write(row))
	must(writer.Write(row)) // 重复行
	row[0] = ""             // 失败行
	must(writer.Write(row))
	writer.Flush()
	must(writer.Error())
	must(form.Close())

	var imported dto.AlumniImportResult
	call(http.MethodPost, "/api/v1/admin/alumni/import", operator.ID, form.FormDataContentType(), &upload, http.StatusOK, &imported)
	if imported.Success != 201 || len(imported.Errors) != 2 {
		t.Fatalf("unexpected import result: %+v", imported)
	}
	var sameNameIDs []uint64
	must(db.Model(&model.AlumniProfile{}).Where("name = ?", sameName).Order("id").Pluck("id", &sameNameIDs).Error)
	if len(sameNameIDs) != 2 {
		t.Fatalf("expected two distinct profiles with the same name, got %v", sameNameIDs)
	}
	var last model.AlumniProfile
	must(db.Where("name = ?", lastName).First(&last).Error)
	// MPA 管理员能在当前名册里看到同一人，但无权看本科导入事件。
	must(db.Create(&model.AlumniEnrollment{
		AlumniID: last.ID, DataDomainID: mpa.ID, TrainingLevel: common.TrainingLevelMaster, Grade: "2025级",
		Status: common.AlumniStatusActive, Version: 1,
	}).Error)
	var batch model.OperationLog
	must(db.Where("operator_id = ? AND target_type = ?", operator.ID, "alumni_batch").Take(&batch).Error)

	legacyJSON := fmt.Sprintf(`{"data_domain_ids":[%d],"batch_created_alumni":[{"id":%d,"name":%q,"field_values":{"grade":"2020级","mobile":"应被过滤"}}]}`,
		mpa.ID, sameNameIDs[0], sameName)
	legacy := model.OperationLog{OperatorID: operator.ID, OperatorRole: operator.Role, Action: "import",
		TargetType: "alumni_batch", Detail: &legacyJSON, CreatedAt: batch.CreatedAt.Add(-time.Minute)}
	must(db.Create(&legacy).Error)

	const path = "/api/v1/admin/audit/operations"
	listPath := func(id uint64) string { return fmt.Sprintf("%s?target_id=%d", path, id) }
	detailPath := func(logID, id uint64) string { return fmt.Sprintf("%s/%d?target_id=%d", path, logID, id) }
	var list dto.AuditOperationPage
	call(http.MethodGet, listPath(last.ID), undergraduateAdmin.ID, "", nil, http.StatusOK, &list)
	if list.Total != 1 || len(list.Items) != 1 || list.Items[0].ID != batch.ID ||
		list.Items[0].Action != "import" || list.Items[0].TargetID == nil || *list.Items[0].TargetID != last.ID {
		t.Fatalf("201st profile must show its import event: %+v", list)
	}
	call(http.MethodGet, listPath(last.ID), mpaAdmin.ID, "", nil, http.StatusOK, &list)
	if list.Total != 0 {
		t.Fatalf("MPA admin received a cross-domain import: %+v", list)
	}
	call(http.MethodGet, listPath(sameNameIDs[0]), mpaAdmin.ID, "", nil, http.StatusOK, &list)
	if list.Total != 2 {
		t.Fatalf("old and new events should be listed: %+v", list)
	}
	call(http.MethodGet, listPath(sameNameIDs[1]), mpaAdmin.ID, "", nil, http.StatusOK, &list)
	if list.Total != 1 || list.Items[0].ID != batch.ID {
		t.Fatalf("legacy association matched a different profile with the same name: %+v", list)
	}

	var detail dto.AuditOperation
	call(http.MethodGet, detailPath(legacy.ID, sameNameIDs[0]), mpaAdmin.ID, "", nil, http.StatusOK, &detail)
	if len(detail.BatchCreatedAlumni) != 1 || detail.BatchCreatedAlumni[0].ID != sameNameIDs[0] ||
		detail.BatchCreatedAlumni[0].FieldValues["grade"] != "2020级" || detail.BatchCreatedAlumni[0].FieldValues["mobile"] != "" {
		t.Fatalf("legacy detail must show only this profile's public snapshot: %+v", detail)
	}
	detail = dto.AuditOperation{}
	call(http.MethodGet, detailPath(batch.ID, last.ID), undergraduateAdmin.ID, "", nil, http.StatusOK, &detail)
	if detail.TargetID == nil || *detail.TargetID != last.ID || len(detail.BatchCreatedAlumni) != 0 || detail.TargetMeta != "" {
		t.Fatalf("scoped detail without a snapshot must not reveal other members: %+v", detail)
	}
	detail = dto.AuditOperation{}
	call(http.MethodGet, fmt.Sprintf("%s/%d", path, batch.ID), operator.ID, "", nil, http.StatusOK, &detail)
	if len(detail.BatchCreatedAlumni) != 200 || detail.BatchCreatedAlumniTotal != 201 || !detail.BatchCreatedAlumniTruncated {
		t.Fatalf("unexpected global batch snapshot: %d rows, total %d", len(detail.BatchCreatedAlumni), detail.BatchCreatedAlumniTotal)
	}
	call(http.MethodGet, listPath(last.ID), 0, "", nil, http.StatusUnauthorized, nil)
	call(http.MethodGet, detailPath(batch.ID, last.ID), alumniUser.ID, "", nil, http.StatusForbidden, nil)
	call(http.MethodGet, detailPath(batch.ID, last.ID), mpaAdmin.ID, "", nil, http.StatusNotFound, nil)
	call(http.MethodGet, detailPath(legacy.ID, sameNameIDs[1]), operator.ID, "", nil, http.StatusNotFound, nil)
	call(http.MethodGet, detailPath(batch.ID, 0), operator.ID, "", nil, http.StatusBadRequest, nil)
}
