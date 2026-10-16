// Package docs_test 校验接口注解与实际注册的路由一致。
//
// 这里故意不读取已提交的 swagger.json，而是用 swag 的生成器按当前注解现场重新生成，
// 否则「新增路由但忘记写注解」时旧规范仍然自洽，测试不会失败。
package docs_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/router"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/swaggo/swag/gen"
	"go.uber.org/zap"
)

type operation struct {
	Method string
	Path   string
}

func (o operation) String() string {
	return o.Method + " " + o.Path
}

// silentDebugger 丢弃生成器的进度输出，保持测试输出干净。
type silentDebugger struct{}

func (silentDebugger) Printf(string, ...interface{}) {}

// generateSpec 按当前 handler 注解重新生成接口规范并返回其内容。
func generateSpec(t *testing.T) []byte {
	t.Helper()

	outputDir := t.TempDir()
	generator := gen.New()

	err := generator.Build(&gen.Config{
		Debugger:           silentDebugger{},
		SearchDir:          filepath.Join("..", ".."),
		MainAPIFile:        "cmd/api/main.go",
		OutputDir:          outputDir,
		OutputTypes:        []string{"json"},
		PropNamingStrategy: "camelcase",
		ParseDepth:         100,
	})
	if err != nil {
		t.Fatalf("生成接口规范失败：%v", err)
	}

	content, err := os.ReadFile(filepath.Join(outputDir, "swagger.json"))
	if err != nil {
		t.Fatalf("读取生成的接口规范失败：%v", err)
	}

	return content
}

// documentedOperations 返回规范中声明的全部接口。
func documentedOperations(t *testing.T, spec []byte) map[operation]bool {
	t.Helper()

	var parsed struct {
		BasePath string `json:"basePath"`
		Paths    map[string]map[string]json.RawMessage
	}
	if err := json.Unmarshal(spec, &parsed); err != nil {
		t.Fatalf("解析接口规范失败：%v", err)
	}
	if parsed.BasePath == "" {
		t.Fatal("接口规范缺少 basePath")
	}

	operations := make(map[operation]bool)
	for path, item := range parsed.Paths {
		for method := range item {
			switch strings.ToUpper(method) {
			case "GET", "POST", "PUT", "DELETE", "PATCH":
			default:
				continue
			}

			operations[operation{
				Method: strings.ToUpper(method),
				Path:   normalizeSpecPath(parsed.BasePath + path),
			}] = true
		}
	}

	return operations
}

// normalizeSpecPath 把规范中的 {id} 占位符转换为 Gin 的 :id 形式。
func normalizeSpecPath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			parts[i] = ":" + strings.Trim(part, "{}")
		}
	}

	return strings.Join(parts, "/")
}

// registeredOperations 返回路由中实际注册的业务接口。
func registeredOperations(engine *gin.Engine) map[operation]bool {
	operations := make(map[operation]bool)
	for _, route := range engine.Routes() {
		if !strings.HasPrefix(route.Path, "/api/v1/") {
			continue
		}

		operations[operation{Method: route.Method, Path: route.Path}] = true
	}

	return operations
}

func newEngine(t *testing.T, storageClient *storage.Client) *gin.Engine {
	t.Helper()

	return router.New(router.Dependencies{
		Config: config.Config{
			App: config.AppConfig{Name: "test-api", Env: config.EnvDevelopment},
			Server: config.ServerConfig{
				Host:              "127.0.0.1",
				Port:              8080,
				ReadHeaderTimeout: time.Second,
				ShutdownTimeout:   time.Second,
			},
			Swagger: config.SwaggerConfig{Enabled: true},
		},
		Logger:        zap.NewNop(),
		StorageClient: storageClient,
	})
}

// storageOnlyRoutes 是仅在启用对象存储时才注册的接口，
// 关闭存储时规范中存在但路由不存在属于预期差异。
var storageOnlyRoutes = map[operation]bool{
	{Method: "GET", Path: "/api/v1/admin/alumni/:id/files"}:                  true,
	{Method: "POST", Path: "/api/v1/admin/alumni/:id/files/upload-url"}:      true,
	{Method: "POST", Path: "/api/v1/admin/alumni/:id/files/:fileId/confirm"}: true,
	{Method: "GET", Path: "/api/v1/admin/alumni/:id/files/:fileId/download"}: true,
	{Method: "DELETE", Path: "/api/v1/admin/alumni/:id/files/:fileId"}:       true,
}

func assertOperationsMatch(t *testing.T, engine *gin.Engine, allowedMissing map[operation]bool) {
	t.Helper()

	documented := documentedOperations(t, generateSpec(t))
	registered := registeredOperations(engine)

	var missing, extra []string
	for op := range registered {
		if !documented[op] {
			missing = append(missing, op.String())
		}
	}
	for op := range documented {
		if !registered[op] && !allowedMissing[op] {
			extra = append(extra, op.String())
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("以下接口已注册但没有接口注解，Swagger UI 中不会出现：\n  %s",
			strings.Join(missing, "\n  "))
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		t.Errorf("以下接口有接口注解但未注册路由，请删除注解或补上路由：\n  %s",
			strings.Join(extra, "\n  "))
	}
}

// TestSpecCoversEveryRoute 保证路由注册与接口注解双向一致。
// 新增或修改路由却没有同步注解时，此测试失败并指出具体的 METHOD + 路径。
func TestSpecCoversEveryRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("未启用对象存储", func(t *testing.T) {
		assertOperationsMatch(t, newEngine(t, nil), storageOnlyRoutes)
	})

	t.Run("启用对象存储", func(t *testing.T) {
		assertOperationsMatch(t, newEngine(t, &storage.Client{}), nil)
	})
}
