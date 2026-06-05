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
}

func TestLoadUsesEnvironmentOverrides(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("AUTH_JWT_SECRET", "test-secret")
	t.Setenv("APP_NAME", "override-api")
	t.Setenv("SERVER_PORT", "9090")
	t.Setenv("DB_ENABLED", "true")
	t.Setenv("REDIS_ENABLED", "true")
	t.Setenv("REDIS_READ_TIMEOUT", "1500ms")
	t.Setenv("AUTH_ACCESS_TOKEN_TTL", "2h")

	cfg, _ := Load()

	if cfg.App.Name != "override-api" {
		t.Fatalf("expected app name override, got %q", cfg.App.Name)
	}
	if cfg.Server.Port != 9090 {
		t.Fatalf("expected server port override, got %d", cfg.Server.Port)
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
	} {
		t.Setenv(key, "")
	}
}
