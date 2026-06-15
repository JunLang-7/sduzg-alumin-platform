package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
	"github.com/gin-gonic/gin"
)

func TestCORSDisabledDoesNotSetHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(CORS(config.CORSConfig{Enabled: false}))
	engine.GET("/api/v1/health/live", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/live", nil)
	req.Header.Set("Origin", "http://127.0.0.1:5173")
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("expected no access control allow origin header, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSAllowsConfiguredOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(CORS(config.CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"http://127.0.0.1:5173", "https://h5.example.com"},
	}))
	engine.GET("/api/v1/health/live", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/live", nil)
	req.Header.Set("Origin", "https://h5.example.com")
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "https://h5.example.com" {
		t.Fatalf("expected configured allow origin, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("expected credentials allowed, got %q", rec.Header().Get("Access-Control-Allow-Credentials"))
	}
	if rec.Header().Get("Vary") != "Origin" {
		t.Fatalf("expected vary origin header, got %q", rec.Header().Get("Vary"))
	}
}

func TestCORSRejectsUnconfiguredOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(CORS(config.CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"https://h5.example.com"},
	}))
	engine.GET("/api/v1/health/live", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/live", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("expected no allow origin for unconfigured origin, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSPreflightReturnsNoContent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(CORS(config.CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"https://h5.example.com"},
	}))
	engine.GET("/api/v1/alumni", func(c *gin.Context) {
		t.Fatal("preflight should abort before handler")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/alumni", nil)
	req.Header.Set("Origin", "https://h5.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "https://h5.example.com" {
		t.Fatalf("expected configured allow origin, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if rec.Header().Get("Access-Control-Allow-Methods") != "GET,POST,PUT,PATCH,DELETE,OPTIONS" {
		t.Fatalf("expected allow methods header, got %q", rec.Header().Get("Access-Control-Allow-Methods"))
	}
	if rec.Header().Get("Access-Control-Allow-Headers") != "Origin,Content-Type,Authorization,X-Request-Id" {
		t.Fatalf("expected allow headers header, got %q", rec.Header().Get("Access-Control-Allow-Headers"))
	}
}

func TestCORSPreflightRejectsUnconfiguredOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(CORS(config.CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"https://h5.example.com"},
	}))
	engine.PATCH("/api/v1/alumni/me", func(c *gin.Context) {
		t.Fatal("rejected preflight should abort before handler")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/alumni/me", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodPatch)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("expected no allow origin for rejected preflight, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}
