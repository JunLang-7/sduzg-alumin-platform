package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func newSwaggerEngine(t *testing.T, enabled bool) *gin.Engine {
	t.Helper()

	return New(Dependencies{
		Config: config.Config{
			App: config.AppConfig{Name: "test-api", Env: config.EnvDevelopment},
			Server: config.ServerConfig{
				Host:              "127.0.0.1",
				Port:              8080,
				ReadHeaderTimeout: time.Second,
				ShutdownTimeout:   time.Second,
			},
			Swagger: config.SwaggerConfig{Enabled: enabled},
		},
		Logger: zap.NewNop(),
	})
}

// TestSwaggerUIPrefixRedirects 保证手工输入文档前缀时不会看到 404。
// gin-swagger 只按文件名匹配，裸前缀本会被它当作未知文件返回 404，
// 因此 /swagger 与 /swagger/ 必须跳转到入口页。
func TestSwaggerUIPrefixRedirects(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := newSwaggerEngine(t, true)

	for _, path := range []string{"/swagger", "/swagger/"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

			if rec.Code != http.StatusFound {
				t.Fatalf("期望 %d 重定向，实际 %d，正文 %q",
					http.StatusFound, rec.Code, rec.Body.String())
			}
			if location := rec.Header().Get("Location"); location != "/swagger/index.html" {
				t.Fatalf("期望跳转到 /swagger/index.html，实际 %q", location)
			}
		})
	}
}

// TestSwaggerUIAssets 保证入口页与规范端点可直接访问。
func TestSwaggerUIAssets(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := newSwaggerEngine(t, true)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("入口页期望状态 200，实际 %d", rec.Code)
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "swagger") {
		t.Fatalf("入口页内容不含 swagger 界面，正文 %q", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("规范端点期望状态 200，实际 %d", rec.Code)
	}

	var spec struct {
		BasePath string `json:"basePath"`
		Paths    map[string]any
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
		t.Fatalf("规范端点返回的不是合法 JSON：%v", err)
	}
	if spec.BasePath != "/api/v1" {
		t.Fatalf("期望 basePath 为 /api/v1，实际 %q", spec.BasePath)
	}
	if len(spec.Paths) == 0 {
		t.Fatal("规范的 paths 为空")
	}
}

// TestSwaggerUIDisabled 保证开关关闭时文档入口整体不可访问。
func TestSwaggerUIDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := newSwaggerEngine(t, false)

	for _, path := range []string{"/swagger", "/swagger/", "/swagger/index.html", "/swagger/doc.json"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

			if rec.Code != http.StatusNotFound {
				t.Fatalf("关闭开关时 %s 期望 404，实际 %d", path, rec.Code)
			}
		})
	}
}
