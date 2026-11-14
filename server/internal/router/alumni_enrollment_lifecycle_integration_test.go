package router

import (
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
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// TestAlumniEnrollmentLifecycleAPI 覆盖学籍独立删除与培养类别更正的完整 HTTP 路径：
// 软删除、撤销语义（未提交即不删除）、最后一段保护、层次与类别组合校验、跨域权限与版本冲突。
func TestAlumniEnrollmentLifecycleAPI(t *testing.T) {
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

	var mpa, academic model.DataDomain
	if err := db.Where("code = ?", common.DataDomainMPA).First(&mpa).Error; err != nil {
		t.Fatalf("find MPA domain: %v", err)
	}
	if err := db.Where("code = ?", common.DataDomainAcademicGraduate).First(&academic).Error; err != nil {
		t.Fatalf("find academic domain: %v", err)
	}

	tag := fmt.Sprintf("lifecycle-%d", time.Now().UnixNano())
	var userIDs, profileIDs []uint64
	t.Cleanup(func() {
		for _, cleanup := range []struct {
			model any
			field string
			ids   []uint64
		}{
			{&model.OperationLog{}, "operator_id", userIDs},
			{&model.AdminDataScope{}, "user_id", userIDs},
			{&model.AdminPermission{}, "user_id", userIDs},
			{&model.User{}, "id", userIDs},
			{&model.AlumniEnrollment{}, "alumni_id", profileIDs},
			{&model.AlumniProfile{}, "id", profileIDs},
		} {
			if len(cleanup.ids) > 0 {
				if err := db.Unscoped().Where(cleanup.field+" IN ?", cleanup.ids).Delete(cleanup.model).Error; err != nil {
					t.Errorf("clean up fixture %T: %v", cleanup.model, err)
				}
			}
		}
	})

	newProfile := func(label string) *model.AlumniProfile {
		profile := &model.AlumniProfile{Name: tag + "-" + label, Status: common.AlumniStatusActive, Version: 1}
		if err := db.Create(profile).Error; err != nil {
			t.Fatalf("create profile: %v", err)
		}
		profileIDs = append(profileIDs, profile.ID)
		return profile
	}
	enroll := func(profile *model.AlumniProfile, domainID uint64, level, grade string) *model.AlumniEnrollment {
		enrollment := &model.AlumniEnrollment{
			AlumniID: profile.ID, DataDomainID: domainID, TrainingLevel: level, Grade: grade,
			Status: common.AlumniStatusActive, Version: 1,
		}
		if err := db.Create(enrollment).Error; err != nil {
			t.Fatalf("create enrollment: %v", err)
		}
		return enrollment
	}

	multi := newProfile("multi")
	mpaEnrollment := enroll(multi, mpa.ID, common.TrainingLevelMaster, "2014")
	academicEnrollment := enroll(multi, academic.ID, common.TrainingLevelDoctor, "2020")

	single := newProfile("single")
	singleEnrollment := enroll(single, mpa.ID, common.TrainingLevelMaster, "2016")

	hiddenOnly := newProfile("hidden-only")
	hiddenMpaEnrollment := enroll(hiddenOnly, mpa.ID, common.TrainingLevelMaster, "2015")
	enroll(hiddenOnly, academic.ID, common.TrainingLevelDoctor, "2021")

	users := map[string]*model.User{
		"super-admin": {Role: common.RoleSuperAdmin},
		"dual-admin":  {Role: common.RoleAdmin},
		"mpa-admin":   {Role: common.RoleAdmin},
	}
	for label, user := range users {
		user.Account = tag + "-" + label
		user.PasswordHash = "not-used-by-lifecycle-test"
		user.Status = common.UserStatusActive
		if err := db.Create(user).Error; err != nil {
			t.Fatalf("create user: %v", err)
		}
		userIDs = append(userIDs, user.ID)
	}
	for _, scope := range []struct {
		label  string
		domain uint64
	}{
		{"dual-admin", mpa.ID},
		{"dual-admin", academic.ID},
		{"mpa-admin", mpa.ID},
	} {
		if err := db.Create(&model.AdminDataScope{UserID: users[scope.label].ID, DataDomainID: scope.domain}).Error; err != nil {
			t.Fatalf("grant data scope: %v", err)
		}
	}

	gin.SetMode(gin.TestMode)
	secret := "lifecycle-test-secret"
	engine := New(Dependencies{
		Config: config.Config{
			App:  config.AppConfig{Name: "lifecycle-test", Env: config.EnvDevelopment},
			Auth: config.AuthConfig{JWTSecret: secret, AccessTokenTTL: time.Hour},
		},
		Logger: zap.NewNop(),
		DB:     db,
	})
	tokens := map[string]string{}
	for label, user := range users {
		tokens[label] = testAccessTokenForUser(t, secret, user.ID, time.Now().Add(time.Hour))
	}
	updatePath := func(profile *model.AlumniProfile) string {
		return fmt.Sprintf("/api/v1/admin/alumni/%d", profile.ID)
	}

	readDetail := func(profile *model.AlumniProfile, token string) dto.AlumniDetail {
		t.Helper()
		rec := performAPIRequest(t, engine, http.MethodGet, fmt.Sprintf("/api/v1/alumni/%d", profile.ID), "", token)
		var body struct {
			Code int              `json:"code"`
			Data dto.AlumniDetail `json:"data"`
		}
		decodeAPIResponse(t, rec, &body)
		if rec.Code != http.StatusOK || body.Code != 0 {
			t.Fatalf("detail unavailable: %d %s", rec.Code, rec.Body.String())
		}
		return body.Data
	}

	t.Run("删除单段学籍只在保存后生效并保留其他学段", func(t *testing.T) {
		payload := fmt.Sprintf(`{
			"version": %d,
			"name": %q,
			"enrollments": [
				{"id": %d, "version": %d, "deleted": true},
				{"id": %d, "version": %d, "data_domain_id": %d, "training_level": "doctor", "grade": "2020级"}
			]
		}`, multi.Version, multi.Name, mpaEnrollment.ID, mpaEnrollment.Version, academicEnrollment.ID, academicEnrollment.Version, academic.ID)
		rec := performAPIRequest(t, engine, http.MethodPut, updatePath(multi), payload, tokens["super-admin"])
		var body struct {
			Code int              `json:"code"`
			Data dto.AlumniDetail `json:"data"`
		}
		decodeAPIResponse(t, rec, &body)
		if rec.Code != http.StatusOK || body.Code != 0 {
			t.Fatalf("delete enrollment failed: %d %s", rec.Code, rec.Body.String())
		}
		if len(body.Data.Enrollments) != 1 || body.Data.Enrollments[0].ID != academicEnrollment.ID {
			t.Fatalf("unexpected remaining enrollments: %+v", body.Data.Enrollments)
		}
		detail := readDetail(multi, tokens["super-admin"])
		if len(detail.Enrollments) != 1 {
			t.Fatalf("deleted enrollment still visible: %+v", detail.Enrollments)
		}
		var deleted model.AlumniEnrollment
		if err := db.Unscoped().First(&deleted, mpaEnrollment.ID).Error; err != nil {
			t.Fatalf("reload deleted enrollment: %v", err)
		}
		if deleted.Status != common.AlumniStatusDeleted || !deleted.DeletedAt.Valid {
			t.Fatalf("enrollment not soft deleted: %+v", deleted)
		}
	})

	t.Run("未提交的学籍不会被当作删除", func(t *testing.T) {
		detail := readDetail(hiddenOnly, tokens["super-admin"])
		payload := fmt.Sprintf(`{
			"version": %d,
			"name": %q,
			"enrollments": [
				{"id": %d, "version": %d, "data_domain_id": %d, "training_level": "master", "grade": "2015级"}
			]
		}`, detail.Version, detail.Name, hiddenMpaEnrollment.ID, enrollmentVersion(t, db, hiddenMpaEnrollment.ID), mpa.ID)
		rec := performAPIRequest(t, engine, http.MethodPut, updatePath(hiddenOnly), payload, tokens["super-admin"])
		if rec.Code != http.StatusOK {
			t.Fatalf("partial update failed: %d %s", rec.Code, rec.Body.String())
		}
		var activeCount int64
		if err := db.Model(&model.AlumniEnrollment{}).
			Where("alumni_id = ?", hiddenOnly.ID).Count(&activeCount).Error; err != nil {
			t.Fatal(err)
		}
		if activeCount != 2 {
			t.Fatalf("omitted enrollment was deleted, active count = %d", activeCount)
		}
	})

	t.Run("清空学籍数组不会删除任何学籍", func(t *testing.T) {
		detail := readDetail(multi, tokens["super-admin"])
		rec := performAPIRequest(t, engine, http.MethodPut, updatePath(multi), fmt.Sprintf(`{
			"version": %d,
			"name": %q,
			"enrollments": []
		}`, detail.Version, detail.Name), tokens["super-admin"])
		if rec.Code != http.StatusOK {
			t.Fatalf("empty enrollment array failed: %d %s", rec.Code, rec.Body.String())
		}
		var active int64
		if err := db.Model(&model.AlumniEnrollment{}).Where("alumni_id = ?", multi.ID).Count(&active).Error; err != nil {
			t.Fatal(err)
		}
		if active != 1 {
			t.Fatalf("empty array changed enrollments, active count = %d", active)
		}
	})

	t.Run("删除最后一段有效学籍返回 409", func(t *testing.T) {
		payload := fmt.Sprintf(`{
			"version": %d,
			"name": %q,
			"enrollments": [{"id": %d, "version": %d, "deleted": true}]
		}`, single.Version, single.Name, singleEnrollment.ID, enrollmentVersion(t, db, singleEnrollment.ID))
		rec := performAPIRequest(t, engine, http.MethodPut, updatePath(single), payload, tokens["super-admin"])
		var body struct {
			Code int `json:"code"`
		}
		decodeAPIResponse(t, rec, &body)
		if rec.Code != http.StatusConflict || body.Code != response.CodeConflict {
			t.Fatalf("expected 409 for last enrollment, got %d %s", rec.Code, rec.Body.String())
		}
		if active := enrollmentStatus(t, db, singleEnrollment.ID); active != common.AlumniStatusActive {
			t.Fatalf("last enrollment was deleted despite 409: %q", active)
		}
	})

	t.Run("培养层次与培养类别组合不合法返回 400", func(t *testing.T) {
		rec := performAPIRequest(t, engine, http.MethodPut, updatePath(single), fmt.Sprintf(`{
			"version": %d,
			"name": %q,
			"enrollments": [{"id": %d, "version": %d, "data_domain_id": %d, "training_level": "doctor", "grade": "2016级"}]
		}`, single.Version, single.Name, singleEnrollment.ID, enrollmentVersion(t, db, singleEnrollment.ID), mpa.ID), tokens["super-admin"])
		var body struct {
			Code int `json:"code"`
		}
		decodeAPIResponse(t, rec, &body)
		if rec.Code != http.StatusBadRequest || body.Code != response.CodeBadRequest {
			t.Fatalf("expected 400 for invalid level/category, got %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("单域管理员不能把学籍转到无权管理的类别", func(t *testing.T) {
		rec := performAPIRequest(t, engine, http.MethodPut, updatePath(multi), fmt.Sprintf(`{
			"version": %d,
			"name": %q,
			"enrollments": [{"id": %d, "version": %d, "data_domain_id": %d, "training_level": "doctor", "grade": "2020级"}]
		}`, multi.Version, multi.Name, academicEnrollment.ID, enrollmentVersion(t, db, academicEnrollment.ID), mpa.ID), tokens["mpa-admin"])
		if rec.Code != http.StatusNotFound {
			t.Fatalf("hidden enrollment correction should be rejected, got %d %s", rec.Code, rec.Body.String())
		}
		rec = performAPIRequest(t, engine, http.MethodPut, updatePath(single), fmt.Sprintf(`{
			"version": %d,
			"name": %q,
			"enrollments": [{"id": %d, "version": %d, "data_domain_id": %d, "training_level": "master", "grade": "2016级"}]
		}`, single.Version, single.Name, singleEnrollment.ID, enrollmentVersion(t, db, singleEnrollment.ID), academic.ID), tokens["mpa-admin"])
		if rec.Code != http.StatusForbidden {
			t.Fatalf("cross-domain correction should be forbidden, got %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("双域管理员更正类别保留同一学籍", func(t *testing.T) {
		rec := performAPIRequest(t, engine, http.MethodPut, updatePath(single), fmt.Sprintf(`{
			"version": %d,
			"name": %q,
			"enrollments": [{"id": %d, "version": %d, "data_domain_id": %d, "training_level": "master", "grade": "2016级"}]
		}`, single.Version, single.Name, singleEnrollment.ID, enrollmentVersion(t, db, singleEnrollment.ID), academic.ID), tokens["dual-admin"])
		var body struct {
			Code int              `json:"code"`
			Data dto.AlumniDetail `json:"data"`
		}
		decodeAPIResponse(t, rec, &body)
		if rec.Code != http.StatusOK || len(body.Data.Enrollments) != 1 {
			t.Fatalf("category correction failed: %d %s", rec.Code, rec.Body.String())
		}
		corrected := body.Data.Enrollments[0]
		if corrected.ID != singleEnrollment.ID || corrected.DataDomainID != academic.ID {
			t.Fatalf("correction changed the enrollment identity: %+v", corrected)
		}
		// 更正后按新类别授权：原类别管理员不再能访问该校友，目标类别管理员仍可读。
		assertAPIStatus(t, engine, http.MethodGet, fmt.Sprintf("/api/v1/alumni/%d", single.ID), "", tokens["mpa-admin"], http.StatusNotFound)
		assertAPIStatus(t, engine, http.MethodGet, fmt.Sprintf("/api/v1/alumni/%d", single.ID), "", tokens["dual-admin"], http.StatusOK)
	})

	t.Run("过期版本删除返回 409 且不删除", func(t *testing.T) {
		detail := readDetail(hiddenOnly, tokens["super-admin"])
		payload := fmt.Sprintf(`{
			"version": %d,
			"name": %q,
			"enrollments": [{"id": %d, "version": 1, "deleted": true}]
		}`, detail.Version, detail.Name, hiddenMpaEnrollment.ID)
		// 先提升数据库版本，模拟另一名管理员刚保存过该段。
		if err := db.Model(&model.AlumniEnrollment{}).Where("id = ?", hiddenMpaEnrollment.ID).
			Update("version", gorm.Expr("version + 1")).Error; err != nil {
			t.Fatal(err)
		}
		rec := performAPIRequest(t, engine, http.MethodPut, updatePath(hiddenOnly), payload, tokens["super-admin"])
		var body struct {
			Code int `json:"code"`
		}
		decodeAPIResponse(t, rec, &body)
		if rec.Code != http.StatusConflict {
			t.Fatalf("expected version conflict, got %d %s", rec.Code, rec.Body.String())
		}
		if status := enrollmentStatus(t, db, hiddenMpaEnrollment.ID); status != common.AlumniStatusActive {
			t.Fatalf("enrollment deleted despite conflict: %q", status)
		}
	})

	t.Run("删除自己最后可见学籍后正常返回列表", func(t *testing.T) {
		detail := readDetail(hiddenOnly, tokens["mpa-admin"])
		if len(detail.Enrollments) != 1 || detail.Enrollments[0].ID != hiddenMpaEnrollment.ID {
			t.Fatalf("mpa admin should only see the MPA enrollment: %+v", detail.Enrollments)
		}
		rec := performAPIRequest(t, engine, http.MethodPut, updatePath(hiddenOnly), fmt.Sprintf(`{
			"version": %d,
			"name": %q,
			"enrollments": [{"id": %d, "version": %d, "deleted": true}]
		}`, detail.Version, detail.Name, hiddenMpaEnrollment.ID, enrollmentVersion(t, db, hiddenMpaEnrollment.ID)), tokens["mpa-admin"])
		var body struct {
			Code int               `json:"code"`
			Data *dto.AlumniDetail `json:"data"`
		}
		decodeAPIResponse(t, rec, &body)
		if rec.Code != http.StatusOK || body.Code != 0 || body.Data != nil {
			t.Fatalf("expected success without detail, got %d %s", rec.Code, rec.Body.String())
		}
		if status := enrollmentStatus(t, db, hiddenMpaEnrollment.ID); status != common.AlumniStatusDeleted {
			t.Fatalf("visible enrollment was not deleted: %q", status)
		}
		var hiddenRemaining int64
		if err := db.Model(&model.AlumniEnrollment{}).Where("alumni_id = ?", hiddenOnly.ID).Count(&hiddenRemaining).Error; err != nil {
			t.Fatal(err)
		}
		if hiddenRemaining != 1 {
			t.Fatalf("hidden enrollment should remain, count = %d", hiddenRemaining)
		}
		assertAPIStatus(t, engine, http.MethodGet, fmt.Sprintf("/api/v1/alumni/%d", hiddenOnly.ID), "", tokens["mpa-admin"], http.StatusNotFound)
	})
}

func enrollmentVersion(t *testing.T, db *gorm.DB, id uint64) uint32 {
	t.Helper()
	var enrollment model.AlumniEnrollment
	if err := db.First(&enrollment, id).Error; err != nil {
		t.Fatalf("reload enrollment %d: %v", id, err)
	}
	return enrollment.Version
}

func enrollmentStatus(t *testing.T, db *gorm.DB, id uint64) string {
	t.Helper()
	var enrollment model.AlumniEnrollment
	if err := db.Unscoped().First(&enrollment, id).Error; err != nil {
		t.Fatalf("reload enrollment %d: %v", id, err)
	}
	return enrollment.Status
}
