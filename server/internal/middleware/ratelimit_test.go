package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/limiter"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type fakeRateLimiter struct {
	result limiter.Result
	err    error
	keys   []string
}

func (f *fakeRateLimiter) Allow(_ context.Context, key string, _ limiter.Rule) (limiter.Result, error) {
	f.keys = append(f.keys, key)
	return f.result, f.err
}

func TestRateLimitSkipsHealthCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limiter := &fakeRateLimiter{result: limiter.Result{Allowed: false}}
	engine := gin.New()
	engine.Use(RateLimit(config.RateLimitConfig{
		Enabled:   true,
		GlobalRPM: 1,
	}, limiter, zap.NewNop()))
	engine.GET("/api/v1/health/live", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/live", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if len(limiter.keys) != 0 {
		t.Fatalf("expected limiter not to be called, got keys %v", limiter.keys)
	}
}

func TestRateLimitReturnsTooManyRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limiter := &fakeRateLimiter{result: limiter.Result{
		Allowed:    false,
		RetryAfter: 2 * time.Second,
	}}
	engine := gin.New()
	engine.Use(RateLimit(config.RateLimitConfig{
		Enabled:   true,
		GlobalRPM: 1,
	}, limiter, zap.NewNop()))
	engine.GET("/api/v1/alumni", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/alumni", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}
	if rec.Header().Get("Retry-After") != "2" {
		t.Fatalf("expected Retry-After header 2, got %q", rec.Header().Get("Retry-After"))
	}
	if !strings.Contains(rec.Body.String(), `"code":42900`) {
		t.Fatalf("expected too many requests response, got %s", rec.Body.String())
	}
}

func TestRateLimitFailsOpenWhenLimiterErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limiter := &fakeRateLimiter{err: errors.New("redis unavailable")}
	engine := gin.New()
	engine.Use(RateLimit(config.RateLimitConfig{
		Enabled:   true,
		GlobalRPM: 1,
	}, limiter, zap.NewNop()))
	engine.GET("/api/v1/alumni", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/alumni", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected fail-open status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestRateLimitLoginKeyIncludesAccountAndPreservesBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limiter := &fakeRateLimiter{result: limiter.Result{Allowed: true}}
	engine := gin.New()
	engine.Use(RateLimit(config.RateLimitConfig{
		Enabled:   true,
		AuthRPM:   10,
		GlobalRPM: 100,
	}, limiter, zap.NewNop()))
	engine.POST("/api/v1/auth/login", func(c *gin.Context) {
		var body map[string]string
		if err := json.NewDecoder(c.Request.Body).Decode(&body); err != nil {
			t.Fatalf("expected handler to read preserved body, got %v", err)
		}
		c.Status(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"account":"Admin","password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if len(limiter.keys) != 1 || !strings.Contains(limiter.keys[0], ":account:admin:") {
		t.Fatalf("expected account scoped key, got %v", limiter.keys)
	}
}

func TestRateLimitVerifyCodeKeyIncludesTarget(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limiter := &fakeRateLimiter{result: limiter.Result{Allowed: true}}
	engine := gin.New()
	engine.Use(RateLimit(config.RateLimitConfig{
		Enabled:       true,
		VerifyCodeRPM: 3,
		GlobalRPM:     100,
	}, limiter, zap.NewNop()))
	engine.POST("/api/v1/auth/verify-code/send", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-code/send", bytes.NewBufferString(`{"target":"User@Example.COM"}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if len(limiter.keys) != 1 || !strings.Contains(limiter.keys[0], ":target:user@example.com:") {
		t.Fatalf("expected target scoped key, got %v", limiter.keys)
	}
}
