package router

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
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

func TestAuthLogoutRouteRequiresAuth(t *testing.T) {
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

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":40100`) {
		t.Fatalf("expected unauthorized response, got %s", rec.Body.String())
	}
}

func TestAuthLogoutRouteWithToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := New(Dependencies{
		Config: config.Config{
			App: config.AppConfig{
				Name: "test-api",
				Env:  config.EnvDevelopment,
			},
			Auth: config.AuthConfig{
				JWTSecret: "test-secret",
			},
		},
		Logger: zap.NewNop(),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+testAccessToken(t, "test-secret", time.Now().Add(time.Hour)))
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"logged_out":true`) {
		t.Fatalf("expected logout response, got %s", rec.Body.String())
	}
}

func TestAuthChangePasswordRouteRequiresAuth(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", strings.NewReader(`{"old_password":"oldpass","new_password":"NewPassw1","confirm_password":"NewPassw1"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":40100`) {
		t.Fatalf("expected unauthorized response, got %s", rec.Body.String())
	}
}

func TestAlumniListRouteRequiresAuth(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alumni", nil)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":40100`) {
		t.Fatalf("expected unauthorized response, got %s", rec.Body.String())
	}
}

func TestAlumniListRouteWithoutDatabase(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := New(Dependencies{
		Config: config.Config{
			App: config.AppConfig{
				Name: "test-api",
				Env:  config.EnvDevelopment,
			},
			Auth: config.AuthConfig{
				JWTSecret: "test-secret",
			},
		},
		Logger: zap.NewNop(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alumni?page=1&page_size=20", nil)
	req.Header.Set("Authorization", "Bearer "+testAccessToken(t, "test-secret", time.Now().Add(time.Hour)))
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":50300`) {
		t.Fatalf("expected service unavailable response, got %s", rec.Body.String())
	}
}

func TestAlumniDetailRouteRequiresAuth(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alumni/1", nil)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":40100`) {
		t.Fatalf("expected unauthorized response, got %s", rec.Body.String())
	}
}

func TestAlumniDetailRouteRejectsInvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := New(Dependencies{
		Config: config.Config{
			App: config.AppConfig{
				Name: "test-api",
				Env:  config.EnvDevelopment,
			},
			Auth: config.AuthConfig{
				JWTSecret: "test-secret",
			},
		},
		Logger: zap.NewNop(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alumni/not-a-number", nil)
	req.Header.Set("Authorization", "Bearer "+testAccessToken(t, "test-secret", time.Now().Add(time.Hour)))
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":40000`) {
		t.Fatalf("expected bad request response, got %s", rec.Body.String())
	}
}

func TestAlumniDetailRouteWithoutDatabase(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := New(Dependencies{
		Config: config.Config{
			App: config.AppConfig{
				Name: "test-api",
				Env:  config.EnvDevelopment,
			},
			Auth: config.AuthConfig{
				JWTSecret: "test-secret",
			},
		},
		Logger: zap.NewNop(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alumni/1", nil)
	req.Header.Set("Authorization", "Bearer "+testAccessToken(t, "test-secret", time.Now().Add(time.Hour)))
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":50300`) {
		t.Fatalf("expected service unavailable response, got %s", rec.Body.String())
	}
}

func testAccessToken(t *testing.T, secret string, expiresAt time.Time) string {
	t.Helper()

	encoder := base64.RawURLEncoding
	header, err := json.Marshal(map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	})
	if err != nil {
		t.Fatalf("failed to marshal token header: %v", err)
	}
	payload, err := json.Marshal(map[string]any{
		"uid": 1,
		"exp": expiresAt.Unix(),
	})
	if err != nil {
		t.Fatalf("failed to marshal token payload: %v", err)
	}

	unsigned := encoder.EncodeToString(header) + "." + encoder.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(unsigned)); err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	return unsigned + "." + encoder.EncodeToString(mac.Sum(nil))
}
