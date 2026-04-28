package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestHealthRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := New(Dependencies{
		Config: config.Config{
			App: config.AppConfig{
				Name: "test-api",
				Env:  config.EnvDevelopment,
			},
			Server: config.ServerConfig{
				Host:              "127.0.0.1",
				Port:              8080,
				ReadHeaderTimeout: time.Second,
				ShutdownTimeout:   time.Second,
			},
		},
		Logger: zap.NewNop(),
	})

	for _, path := range []string{"/api/v1/health/live", "/api/v1/health/ready"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()

			engine.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
			}
			if !strings.Contains(rec.Body.String(), `"code":0`) {
				t.Fatalf("expected success response, got %s", rec.Body.String())
			}
		})
	}
}

func TestAuthLoginRouteWithoutDatabase(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := New(Dependencies{
		Config: config.Config{
			App: config.AppConfig{
				Name: "test-api",
				Env:  config.EnvDevelopment,
			},
			Auth: config.AuthConfig{
				JWTSecret:      "test-secret",
				AccessTokenTTL: time.Hour,
			},
		},
		Logger: zap.NewNop(),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"account":"admin","password":"Admin@123456"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":50300`) {
		t.Fatalf("expected service unavailable response, got %s", rec.Body.String())
	}
}

func TestAuthLogoutRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := New(Dependencies{
		Config: config.Config{
			App: config.AppConfig{
				Name: "test-api",
				Env:  config.EnvDevelopment,
			},
		},
		Logger: zap.NewNop(),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"logged_out":true`) {
		t.Fatalf("expected logout response, got %s", rec.Body.String())
	}
}
