package limiter

import (
	"net/http"
	"testing"

	"github.com/JunLang-7/sduzg-alumin-platform/server/internal/config"
)

func TestRuleForRequestSkipsHealthChecks(t *testing.T) {
	cfg := config.RateLimitConfig{
		Enabled:   true,
		GlobalRPM: 120,
	}

	for _, path := range []string{"/api/v1/health/live", "/api/v1/health/ready"} {
		t.Run(path, func(t *testing.T) {
			_, ok := RuleForRequest(http.MethodGet, path, cfg)
			if ok {
				t.Fatalf("expected %s to skip rate limiting", path)
			}
		})
	}
}

func TestRuleForRequestMatchesSpecialRoutes(t *testing.T) {
	cfg := config.RateLimitConfig{
		Enabled:       true,
		GlobalRPM:     120,
		AuthRPM:       10,
		VerifyCodeRPM: 3,
		AdminRPM:      30,
	}

	tests := []struct {
		name  string
		path  string
		limit int
		burst int
	}{
		{name: "auth_login", path: "/api/v1/auth/login", limit: 10, burst: 3},
		{name: "verify_code", path: "/api/v1/auth/verify-code/send", limit: 3, burst: 2},
		{name: "admin", path: "/api/v1/admin/alumni/export", limit: 30, burst: 5},
		{name: "global", path: "/api/v1/alumni", limit: 120, burst: 120},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, ok := RuleForRequest(http.MethodPost, tt.path, cfg)
			if !ok {
				t.Fatal("expected request to be rate limited")
			}
			if rule.Name != tt.name {
				t.Fatalf("expected rule name %q, got %q", tt.name, rule.Name)
			}
			if rule.Limit != tt.limit {
				t.Fatalf("expected limit %d, got %d", tt.limit, rule.Limit)
			}
			if rule.Burst != tt.burst {
				t.Fatalf("expected burst %d, got %d", tt.burst, rule.Burst)
			}
		})
	}
}
