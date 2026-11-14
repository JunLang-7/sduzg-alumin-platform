package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/do"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/middleware"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/model"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/service"
	"github.com/gin-gonic/gin"
)

// lifecycleStore 为 handler 测试提供一段可编辑学籍，并按用例返回保存结果。
type lifecycleStore struct {
	stubAlumniStore
	saveResult *do.AlumniSaveResult
	saveErr    error
}

func (s *lifecycleStore) GetDetail(context.Context, uint64, []uint64) (*do.AlumniRecord, error) {
	return &do.AlumniRecord{
		Profile: &model.AlumniProfile{ID: 42, Name: "李四", Status: common.AlumniStatusActive, Version: 3},
		Enrollments: []*model.AlumniEnrollment{{
			ID: 11, AlumniID: 42, DataDomainID: 2, TrainingLevel: common.TrainingLevelMaster,
			Grade: "2020级", Status: common.AlumniStatusActive, Version: 1,
		}},
	}, nil
}

func (s *lifecycleStore) Save(context.Context, uint64, do.AlumniSaveInput, uint64, []uint64) (*do.AlumniSaveResult, error) {
	if s.saveErr != nil {
		return nil, s.saveErr
	}
	return s.saveResult, nil
}

func withAdminAccess(operator common.AccessContext, handler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(middleware.AccessContextKey, &operator)
		handler(c)
	}
}

func callUpdate(t *testing.T, store *lifecycleStore, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := NewAlumniHandler(service.NewAlumniService(store, nil, stubLifecycleDomains{}))
	operator := common.AccessContext{UserID: 7, Role: common.RoleSuperAdmin}
	engine := gin.New()
	engine.PUT("/admin/alumni/:id", withAdminAccess(operator, h.Update))

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPut, "/admin/alumni/42", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

// stubLifecycleDomains 提供与迁移一致的三个培养类别。
type stubLifecycleDomains struct{}

func (stubLifecycleDomains) ListActiveDataDomains(context.Context) ([]*model.DataDomain, error) {
	return stubLifecycleDomainList(), nil
}
func (stubLifecycleDomains) ListDataDomains(context.Context) ([]*model.DataDomain, error) {
	return stubLifecycleDomainList(), nil
}
func (stubLifecycleDomains) ListAdminDataDomainIDs(context.Context, uint64) ([]uint64, error) {
	return nil, nil
}
func (stubLifecycleDomains) ListAdminPermissionCodes(context.Context, uint64) ([]string, error) {
	return nil, nil
}

func stubLifecycleDomainList() []*model.DataDomain {
	return []*model.DataDomain{
		{ID: 1, Code: common.DataDomainUndergraduate, Name: "本科生", Status: common.DataDomainStatusActive},
		{ID: 2, Code: common.DataDomainMPA, Name: "MPA专业学位研究生", Status: common.DataDomainStatusActive},
		{ID: 3, Code: common.DataDomainAcademicGraduate, Name: "学术学位研究生", Status: common.DataDomainStatusActive},
	}
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	return body
}

func TestUpdateHandlerMapsEnrollmentLifecycleErrors(t *testing.T) {
	cases := []struct {
		name       string
		saveErr    error
		wantStatus int
		wantCode   float64
	}{
		{name: "最后一段有效学籍", saveErr: common.ErrLastEnrollment, wantStatus: http.StatusConflict, wantCode: 40900},
		{name: "层次与类别不匹配", saveErr: common.ErrEnrollmentCategoryMismatch, wantStatus: http.StatusBadRequest, wantCode: 40000},
		{name: "版本冲突", saveErr: common.ErrVersionConflict, wantStatus: http.StatusConflict, wantCode: 40900},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			rec := callUpdate(t, &lifecycleStore{saveErr: testCase.saveErr}, map[string]any{
				"version": 3,
				"name":    "李四",
				"enrollments": []map[string]any{
					{"id": 11, "version": 1, "data_domain_id": 2, "training_level": "master", "grade": "2020级"},
				},
			})
			if rec.Code != testCase.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, testCase.wantStatus, rec.Body.String())
			}
			body := decodeBody(t, rec)
			if body["code"] != testCase.wantCode {
				t.Fatalf("code = %v, want %v", body["code"], testCase.wantCode)
			}
		})
	}
}

func TestUpdateHandlerReturnsNullDataWhenAlumniLeavesScope(t *testing.T) {
	store := &lifecycleStore{saveResult: &do.AlumniSaveResult{
		Visible: false,
		Record: &do.AlumniRecord{
			Profile:     &model.AlumniProfile{ID: 42, Name: "李四", Status: common.AlumniStatusActive, Version: 4},
			Enrollments: []*model.AlumniEnrollment{},
		},
	}}
	rec := callUpdate(t, store, map[string]any{
		"version": 3,
		"name":    "李四",
		"enrollments": []map[string]any{
			{"id": 11, "version": 1, "deleted": true},
		},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["code"] != float64(0) {
		t.Fatalf("code = %v, want 0", body["code"])
	}
	if data, exists := body["data"]; !exists || data != nil {
		t.Fatalf("expected explicit null data for hidden alumni, got %v", body["data"])
	}
}
