package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
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

func TestCORSPreflightRunsBeforeAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := New(Dependencies{
		Config: config.Config{
			App: config.AppConfig{
				Name: "test-api",
				Env:  config.EnvDevelopment,
			},
			CORS: config.CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"https://h5.example.com"},
			},
		},
		Logger: zap.NewNop(),
	})

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/alumni", nil)
	req.Header.Set("Origin", "https://h5.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "https://h5.example.com" {
		t.Fatalf("expected cors allow origin header, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
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

func TestAlumniMeRouteRequiresAuth(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alumni/me", nil)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":40100`) {
		t.Fatalf("expected unauthorized response, got %s", rec.Body.String())
	}
}

func TestAlumniMeRouteWithoutDatabase(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/alumni/me", nil)
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

func TestAlumniMeUpdateRouteRejectsInvalidJSON(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodPut, "/api/v1/alumni/me", strings.NewReader(`{`))
	req.Header.Set("Authorization", "Bearer "+testAccessToken(t, "test-secret", time.Now().Add(time.Hour)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":40000`) {
		t.Fatalf("expected bad request response, got %s", rec.Body.String())
	}
}

func TestAdminAlumniCreateRouteRequiresAuth(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/alumni", strings.NewReader(`{"name":"张三","grade":"2020级"}`))
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

func TestAdminAlumniCreateRouteWithoutDatabase(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/alumni", strings.NewReader(`{"name":"张三","grade":"2020级"}`))
	req.Header.Set("Authorization", "Bearer "+testAccessToken(t, "test-secret", time.Now().Add(time.Hour)))
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

func TestAdminAlumniUpdateRouteWithoutDatabase(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/alumni/1", strings.NewReader(`{"name":"张三","grade":"2020级"}`))
	req.Header.Set("Authorization", "Bearer "+testAccessToken(t, "test-secret", time.Now().Add(time.Hour)))
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

func TestAdminAlumniDeleteRouteWithoutDatabase(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/alumni/1", nil)
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

func TestAdminDashboardOverviewRouteRequiresAuth(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/overview", nil)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":40100`) {
		t.Fatalf("expected unauthorized response, got %s", rec.Body.String())
	}
}

func TestAdminDashboardOverviewRouteWithoutDatabase(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/overview", nil)
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

func TestAdminDashboardDistributionRouteRequiresAuth(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/distribution", nil)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":40100`) {
		t.Fatalf("expected unauthorized response, got %s", rec.Body.String())
	}
}

func TestAdminDashboardDistributionRouteWithoutDatabase(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/distribution?dimension=grade", nil)
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

func TestSuperAdminAdminsListRouteRequiresAuth(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/super-admin/admins?page=1&page_size=20", nil)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":40100`) {
		t.Fatalf("expected unauthorized response, got %s", rec.Body.String())
	}
}

func TestSuperAdminAdminsListRouteWithoutDatabase(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/super-admin/admins?page=1&page_size=20", nil)
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

func TestSuperAdminAdminsCreateRouteRequiresAuth(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodPost, "/api/v1/super-admin/admins", strings.NewReader(`{"account":"manager01","password":"InitPass123"}`))
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

func TestSuperAdminAdminsCreateRouteWithoutDatabase(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodPost, "/api/v1/super-admin/admins", strings.NewReader(`{"account":"manager01","password":"InitPass123"}`))
	req.Header.Set("Authorization", "Bearer "+testAccessToken(t, "test-secret", time.Now().Add(time.Hour)))
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

func TestSuperAdminAdminsDeleteRouteRequiresAuth(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/super-admin/admins/2", nil)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":40100`) {
		t.Fatalf("expected unauthorized response, got %s", rec.Body.String())
	}
}

func TestSuperAdminAdminsDeleteRouteWithoutDatabase(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/super-admin/admins/2", nil)
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

	claims := jwt.MapClaims{
		"uid": float64(1),
		"exp": jwt.NewNumericDate(expiresAt),
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return token
}
