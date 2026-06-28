package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadUsesDefaults(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("AUTH_JWT_SECRET", "test-secret")

	cfg, _ := Load()

	if cfg.App.Name != "sdu-alumni-platform" {
		t.Fatalf("expected default app name, got %q", cfg.App.Name)
	}
	if cfg.Server.Port != 8080 {
		t.Fatalf("expected default server port, got %d", cfg.Server.Port)
	}
	if cfg.Database.Enabled {
		t.Fatal("expected database to be disabled by default")
	}
	if cfg.Redis.Enabled {
		t.Fatal("expected redis to be disabled by default")
	}
	if cfg.Auth.JWTSecret != "test-secret" {
		t.Fatalf("expected auth jwt secret override, got %q", cfg.Auth.JWTSecret)
	}
	if cfg.RateLimit.Enabled {
		t.Fatal("expected rate limit to be disabled by default")
	}
	if cfg.RateLimit.GlobalRPM != 120 {
		t.Fatalf("expected default global rpm 120, got %d", cfg.RateLimit.GlobalRPM)
	}
	if cfg.CORS.Enabled {
		t.Fatal("expected cors to be disabled by default")
	}
	if len(cfg.CORS.AllowedOrigins) != 0 {
		t.Fatalf("expected no cors allowed origins by default, got %#v", cfg.CORS.AllowedOrigins)
	}
	if cfg.SMS.Enabled {
		t.Fatal("expected sms to be disabled by default")
	}
	if cfg.SMS.Region != "ap-beijing" {
		t.Fatalf("expected default sms region ap-beijing, got %q", cfg.SMS.Region)
	}
	if cfg.SMS.Endpoint != "sms.tencentcloudapi.com" {
		t.Fatalf("expected default sms endpoint, got %q", cfg.SMS.Endpoint)
	}
}

func TestLoadUsesEnvironmentOverrides(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("AUTH_JWT_SECRET", "test-secret")
	t.Setenv("APP_NAME", "override-api")
	t.Setenv("SERVER_PORT", "9090")
	t.Setenv("SERVER_TRUSTED_PROXIES", "10.0.0.0/8, 192.168.0.1")
	t.Setenv("DB_ENABLED", "true")
	t.Setenv("REDIS_ENABLED", "true")
	t.Setenv("REDIS_READ_TIMEOUT", "1500ms")
	t.Setenv("AUTH_ACCESS_TOKEN_TTL", "2h")
	t.Setenv("RATE_LIMIT_ENABLED", "true")
	t.Setenv("RATE_LIMIT_AUTH_RPM", "8")
	t.Setenv("CORS_ENABLED", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://127.0.0.1:5173, https://h5.example.com")
	t.Setenv("SMS_ENABLED", "true")
	t.Setenv("SMS_TENCENT_SECRET_ID", "secret-id")
	t.Setenv("SMS_TENCENT_SECRET_KEY", "secret-key")
	t.Setenv("SMS_TENCENT_REGION", "ap-shanghai")
	t.Setenv("SMS_TENCENT_APP_ID", "1400000000")
	t.Setenv("SMS_TENCENT_SIGN_NAME", "山东大学政管学院")
	t.Setenv("SMS_TENCENT_TEMPLATE_ID", "123456")
	t.Setenv("SMS_TENCENT_ENDPOINT", "sms.tencentcloudapi.com")

	cfg, _ := Load()

	if cfg.App.Name != "override-api" {
		t.Fatalf("expected app name override, got %q", cfg.App.Name)
	}
	if cfg.Server.Port != 9090 {
		t.Fatalf("expected server port override, got %d", cfg.Server.Port)
	}
	if len(cfg.Server.TrustedProxies) != 2 || cfg.Server.TrustedProxies[0] != "10.0.0.0/8" || cfg.Server.TrustedProxies[1] != "192.168.0.1" {
		t.Fatalf("expected trusted proxies override, got %#v", cfg.Server.TrustedProxies)
	}
	if !cfg.Database.Enabled {
		t.Fatal("expected database override to enable database")
	}
	if !cfg.Redis.Enabled {
		t.Fatal("expected redis override to enable redis")
	}
	if cfg.Redis.ReadTimeout != 1500*time.Millisecond {
		t.Fatalf("expected redis read timeout override, got %s", cfg.Redis.ReadTimeout)
	}
	if cfg.Auth.AccessTokenTTL != 2*time.Hour {
		t.Fatalf("expected auth access token ttl override, got %s", cfg.Auth.AccessTokenTTL)
	}
	if !cfg.RateLimit.Enabled {
		t.Fatal("expected rate limit override to enable rate limit")
	}
	if cfg.RateLimit.AuthRPM != 8 {
		t.Fatalf("expected auth rpm override, got %d", cfg.RateLimit.AuthRPM)
	}
	if !cfg.CORS.Enabled {
		t.Fatal("expected cors override to enable cors")
	}
	if len(cfg.CORS.AllowedOrigins) != 2 || cfg.CORS.AllowedOrigins[0] != "http://127.0.0.1:5173" || cfg.CORS.AllowedOrigins[1] != "https://h5.example.com" {
		t.Fatalf("expected cors allowed origins override, got %#v", cfg.CORS.AllowedOrigins)
	}
	if !cfg.SMS.Enabled {
		t.Fatal("expected sms override to enable sms")
	}
	if cfg.SMS.SecretID != "secret-id" || cfg.SMS.SecretKey != "secret-key" {
		t.Fatalf("expected sms secret overrides, got %#v", cfg.SMS)
	}
	if cfg.SMS.Region != "ap-shanghai" || cfg.SMS.AppID != "1400000000" || cfg.SMS.TemplateID != "123456" {
		t.Fatalf("expected sms tencent overrides, got %#v", cfg.SMS)
	}
}

func TestLoadReadsEnvFile(t *testing.T) {
	clearConfigEnv(t)

	tempDir := t.TempDir()
	t.Chdir(tempDir)

	content := []byte("APP_NAME=file-api\nSERVER_PORT=7070\nREDIS_ENABLED=true\nAUTH_JWT_SECRET=file-secret\n")
	if err := os.WriteFile(".env", content, 0o600); err != nil {
		t.Fatalf("failed to write env file: %v", err)
	}

	cfg, _ := Load()

	if cfg.App.Name != "file-api" {
		t.Fatalf("expected app name from env file, got %q", cfg.App.Name)
	}
	if cfg.Server.Port != 7070 {
		t.Fatalf("expected server port from env file, got %d", cfg.Server.Port)
	}
	if !cfg.Redis.Enabled {
		t.Fatal("expected redis to be enabled from env file")
	}
}

func clearConfigEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{
		"APP_NAME",
		"APP_ENV",
		"SERVER_HOST",
		"SERVER_PORT",
		"SERVER_READ_HEADER_TIMEOUT",
		"SERVER_SHUTDOWN_TIMEOUT",
		"SERVER_TRUSTED_PROXIES",
		"DB_ENABLED",
		"DB_HOST",
		"DB_PORT",
		"DB_USER",
		"DB_PASSWORD",
		"DB_NAME",
		"DB_PARAMS",
		"DB_MAX_OPEN_CONNS",
		"DB_MAX_IDLE_CONNS",
		"DB_CONN_MAX_LIFETIME",
		"REDIS_ENABLED",
		"REDIS_ADDR",
		"REDIS_PASSWORD",
		"REDIS_DB",
		"REDIS_POOL_SIZE",
		"REDIS_MIN_IDLE_CONNS",
		"REDIS_DIAL_TIMEOUT",
		"REDIS_READ_TIMEOUT",
		"REDIS_WRITE_TIMEOUT",
		"AUTH_JWT_SECRET",
		"AUTH_ACCESS_TOKEN_TTL",
		"STORAGE_ENABLED",
		"STORAGE_ENDPOINT",
		"STORAGE_ACCESS_KEY",
		"STORAGE_SECRET_KEY",
		"STORAGE_BUCKET",
		"STORAGE_USE_SSL",
		"SMS_ENABLED",
		"SMS_TENCENT_SECRET_ID",
		"SMS_TENCENT_SECRET_KEY",
		"SMS_TENCENT_REGION",
		"SMS_TENCENT_APP_ID",
		"SMS_TENCENT_SIGN_NAME",
		"SMS_TENCENT_TEMPLATE_ID",
		"SMS_TENCENT_ENDPOINT",
		"EMAIL_ENABLED",
		"EMAIL_HOST",
		"EMAIL_PORT",
		"EMAIL_USERNAME",
		"EMAIL_PASSWORD",
		"EMAIL_FROM_NAME",
		"RATE_LIMIT_ENABLED",
		"RATE_LIMIT_GLOBAL_RPM",
		"RATE_LIMIT_AUTH_RPM",
		"RATE_LIMIT_VERIFY_CODE_RPM",
		"RATE_LIMIT_ADMIN_RPM",
		"CORS_ENABLED",
		"CORS_ALLOWED_ORIGINS",
	} {
		t.Setenv(key, "")
	}
}
