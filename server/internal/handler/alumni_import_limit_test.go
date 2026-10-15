package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/common"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/middleware"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/service"
)

// 超出单次导入行数上限时，接口必须返回 400 与可读文案。旧实现把该错误落到
// default 分支，用户只看到 500「服务器内部错误」。
func TestImportHandlerRowLimitReturnsBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	svc := service.NewAlumniService(&fakeImportStore{}, nil)
	h := NewAlumniHandler(svc)

	engine := gin.New()
	engine.POST("/admin/alumni/import", func(c *gin.Context) {
		c.Set(middleware.AccessContextKey, &common.AccessContext{UserID: 1, Role: common.RoleSuperAdmin})
		h.Import(c)
	})

	const rowsOverLimit = 5001 // 数据行上限为 5000
	rows := make([][]string, 0, rowsOverLimit)
	for i := 0; i < rowsOverLimit; i++ {
		rows = append(rows, []string{"张三", "2020级"})
	}
	buf, filename := buildMultipartXLSX(t, rows)

	body := &bytes.Buffer{}
	body.WriteString("--boundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"data_domain_id\"\r\n\r\n")
	body.WriteString("1\r\n")
	body.WriteString("--boundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"file\"; filename=\"")
	body.WriteString(filename)
	body.WriteString("\"\r\n")
	body.WriteString("Content-Type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet\r\n\r\n")
	body.Write(buf.Bytes())
	body.WriteString("\r\n--boundary--\r\n")

	req := httptest.NewRequest(http.MethodPost, "/admin/alumni/import", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "单次最多导入") {
		t.Fatalf("expected a readable row limit message, got %s", rec.Body.String())
	}
}
